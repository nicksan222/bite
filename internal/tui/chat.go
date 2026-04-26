package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/nicksan222/bite/internal/ai"
)

// Persister is the minimum surface the TUI needs from the store layer.
// Implemented by *db.Store via small adapter methods (see cli/chat.go).
type Persister interface {
	AppendUser(ctx context.Context, content string) error
	AppendAssistant(ctx context.Context, content string) error
}

// SlashHandler executes a slash command (e.g. "/log_meal …") and returns the
// rendered text for the transcript. An error means the line was malformed or
// the tool itself failed; the TUI surfaces it inline. Handlers must NOT call
// the model — slash commands are deterministic, local-only operations.
type SlashHandler func(ctx context.Context, line string) (string, error)

// Option tweaks a New() call.
type Option func(*model)

// WithSlashHandler enables /-prefixed input. Without it, lines starting with
// "/" are sent to the model verbatim.
func WithSlashHandler(h SlashHandler) Option {
	return func(m *model) { m.slash = h }
}

// New returns a configured *tea.Program. Call Run on it.
// streamOpts are forwarded to every ai.Client.Stream call made during the
// session (e.g. ai.WithTools to enable tool calling).
func New(ctx context.Context, client ai.Streamer, store Persister, history []ai.Message, streamOpts []ai.StreamOption, opts ...Option) *tea.Program {
	m := initialModel(ctx, client, store, history, streamOpts)
	for _, o := range opts {
		o(&m)
	}
	return tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
}

// ─── messages ────────────────────────────────────────────────────────────────

type streamDeltaMsg struct{ delta string }
type streamDoneMsg struct{ full string }
type streamErrMsg struct{ err error }

// ─── model ───────────────────────────────────────────────────────────────────

type model struct {
	ctx        context.Context
	client     ai.Streamer
	store      Persister
	history    []ai.Message
	streamOpts []ai.StreamOption

	viewport viewport.Model
	input    textarea.Model
	spinner  spinner.Model
	renderer *glamour.TermRenderer

	streaming bool
	pending   string // accumulated assistant text for the in-flight turn
	streamCh  <-chan ai.StreamEvent
	slash     SlashHandler

	width, height int
	err           error
}

func initialModel(ctx context.Context, client ai.Streamer, store Persister, history []ai.Message, streamOpts []ai.StreamOption) model {
	ta := textarea.New()
	ta.Placeholder = "What did you eat? Ask about calories, macros, meals…  (Enter sends, Ctrl+C quits)"
	ta.Focus()
	ta.CharLimit = 8000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Prompt = "│ "

	vp := viewport.New(80, 20)
	vp.SetContent(welcomeText())

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)

	return model{
		ctx:        ctx,
		client:     client,
		store:      store,
		history:    history,
		streamOpts: streamOpts,
		input:      ta,
		viewport:   vp,
		spinner:    sp,
		renderer:   r,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m model) Update(raw tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := raw.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		m.viewport.SetContent(m.renderTranscript())

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "ctrl+d":
			return m, tea.Quit
		case "enter":
			if m.streaming {
				return m, nil
			}
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, nil
			}
			m.input.Reset()
			return m.send(text)
		}

	case streamDeltaMsg:
		m.pending += msg.delta
		m.viewport.SetContent(m.renderTranscript())
		m.viewport.GotoBottom()
		return m, m.readNext()

	case streamDoneMsg:
		final := msg.full
		if final == "" {
			final = m.pending
		}
		m.history = append(m.history, ai.Message{Role: ai.RoleAssistant, Content: final})
		if m.store != nil {
			if err := m.store.AppendAssistant(m.ctx, final); err != nil {
				m.err = fmt.Errorf("save message: %w", err)
			}
		}
		m.pending = ""
		m.streaming = false
		m.streamCh = nil
		m.viewport.SetContent(m.renderTranscript())
		m.viewport.GotoBottom()
		return m, nil

	case streamErrMsg:
		m.err = msg.err
		m.streaming = false
		m.streamCh = nil
		m.pending = ""
		m.viewport.SetContent(m.renderTranscript())
		m.viewport.GotoBottom()
		return m, nil

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(raw)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(raw)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	header := headerStyle.Width(m.width).Render("🍽  bite")

	var status string
	switch {
	case m.err != nil:
		status = errorStyle.Render("error: " + m.err.Error())
	case m.streaming:
		status = m.spinner.View() + hintStyle.Render(" thinking…")
	default:
		status = hintStyle.Render("ready • Enter to send • Ctrl+C to quit")
	}

	return strings.Join([]string{
		header,
		m.viewport.View(),
		inputBoxStyle.Width(m.width - 2).Render(m.input.View()),
		"  " + status,
	}, "\n")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func (m *model) layout() {
	w := max(m.width, 20)
	h := max(m.height-9, 5) // header + input box + status + padding

	m.input.SetWidth(w - 4)
	m.viewport.Width = w
	m.viewport.Height = h

	if m.renderer != nil {
		r, _ := glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(w-2),
		)
		m.renderer = r
	}
}

func (m model) renderTranscript() string {
	var b strings.Builder

	for _, msg := range m.history {
		b.WriteString(m.renderTurn(msg.Role, msg.Content))
		b.WriteString("\n")
	}

	if m.streaming || m.pending != "" {
		b.WriteString(m.renderTurn(ai.RoleAssistant, m.pending))
		b.WriteString("\n")
	}

	return b.String()
}

func (m model) renderTurn(role ai.Role, content string) string {
	var label string
	switch role {
	case ai.RoleUser:
		label = userStyle.Render("you")
	case ai.RoleAssistant:
		label = assistantStyle.Render("bite")
	default:
		label = string(role)
	}

	body := content
	if role == ai.RoleAssistant && m.renderer != nil && content != "" {
		if rendered, err := m.renderer.Render(content); err == nil {
			body = strings.TrimRight(rendered, "\n")
		}
	}

	return fmt.Sprintf("%s\n%s", label, body)
}

func (m model) send(text string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(text, "/") && m.slash != nil {
		return m.handleSlash(text)
	}
	m.history = append(m.history, ai.Message{Role: ai.RoleUser, Content: text})
	if m.store != nil {
		if err := m.store.AppendUser(m.ctx, text); err != nil {
			m.err = fmt.Errorf("save message: %w", err)
			// Don't start streaming: the user turn isn't persisted, so an
			// assistant reply would create an orphaned DB row.
			m.viewport.SetContent(m.renderTranscript())
			return m, nil
		}
	}

	ch, err := m.client.Stream(m.ctx, m.history, m.streamOpts...)
	if err != nil {
		m.err = err
		m.viewport.SetContent(m.renderTranscript())
		return m, nil
	}

	m.streamCh = ch
	m.streaming = true
	m.pending = ""
	m.viewport.SetContent(m.renderTranscript())
	m.viewport.GotoBottom()

	return m, tea.Batch(m.readNext(), m.spinner.Tick)
}

// handleSlash executes a slash command locally, injects the result into chat
// history, and skips the model call. The next real user message picks the
// result up in context, so the model stays aware of what just happened.
func (m model) handleSlash(text string) (tea.Model, tea.Cmd) {
	out, err := m.slash(m.ctx, text)
	if err != nil {
		// Render inline error without polluting persisted history.
		m.err = err
		m.viewport.SetContent(m.renderTranscript())
		m.viewport.GotoBottom()
		return m, nil
	}
	m.err = nil
	m.history = append(m.history,
		ai.Message{Role: ai.RoleUser, Content: text},
		ai.Message{Role: ai.RoleAssistant, Content: out},
	)
	if m.store != nil {
		if e := m.store.AppendUser(m.ctx, text); e != nil {
			m.err = fmt.Errorf("save message: %w", e)
		}
		if e := m.store.AppendAssistant(m.ctx, out); e != nil {
			m.err = fmt.Errorf("save message: %w", e)
		}
	}
	m.viewport.SetContent(m.renderTranscript())
	m.viewport.GotoBottom()
	return m, nil
}

func (m model) readNext() tea.Cmd {
	ch := m.streamCh
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return streamDoneMsg{}
		}
		switch {
		case ev.Err != nil:
			return streamErrMsg{err: ev.Err}
		case ev.Done:
			return streamDoneMsg{full: ev.Final}
		default:
			return streamDeltaMsg{delta: ev.Delta}
		}
	}
}

func welcomeText() string {
	return strings.Join([]string{
		assistantStyle.Render("bite"),
		"  your terminal nutritionist. log a meal, ask about calories, or just chat.",
		"  Enter to send · /help for slash commands · Ctrl+C to quit",
	}, "\n")
}
