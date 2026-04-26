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
// Satisfied by tools.ChatPersister so tui never grows direct database
// knowledge.
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
type streamStepMsg struct{ step ai.ToolStep }

// ─── model ───────────────────────────────────────────────────────────────────

// toolStep is the TUI's per-turn record of one tool invocation. It mirrors
// ai.ToolStep but stays inside the TUI package so the renderer can format
// the lines without exposing internal state.
type toolStep struct {
	id       string
	name     string
	args     string
	result   string
	finished bool
}

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

	streaming    bool
	pending      string // accumulated assistant text for the in-flight turn
	pendingSteps []toolStep
	stepsByTurn  map[int][]toolStep // history index → tool steps for that assistant turn
	streamCh     <-chan ai.StreamEvent
	slash        SlashHandler

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
		ctx:         ctx,
		client:      client,
		store:       store,
		history:     history,
		streamOpts:  streamOpts,
		input:       ta,
		viewport:    vp,
		spinner:     sp,
		renderer:    r,
		stepsByTurn: map[int][]toolStep{},
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

	case streamStepMsg:
		m.applyStep(msg.step)
		m.viewport.SetContent(m.renderTranscript())
		m.viewport.GotoBottom()
		return m, m.readNext()

	case streamDoneMsg:
		final := msg.full
		if final == "" {
			final = m.pending
		}
		m.history = append(m.history, ai.Message{Role: ai.RoleAssistant, Content: final})
		if len(m.pendingSteps) > 0 {
			m.stepsByTurn[len(m.history)-1] = m.pendingSteps
			m.pendingSteps = nil
		}
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
		m.pendingSteps = nil
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
		status = hintStyle.Render("ready • Enter to send • /help • Ctrl+C to quit")
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
	var blocks []string

	for i, msg := range m.history {
		var steps []toolStep
		if msg.Role == ai.RoleAssistant {
			steps = m.stepsByTurn[i]
		}
		blocks = append(blocks, m.renderTurn(msg.Role, msg.Content, steps))
	}

	if m.streaming || m.pending != "" || len(m.pendingSteps) > 0 {
		blocks = append(blocks, m.renderTurn(ai.RoleAssistant, m.pending, m.pendingSteps))
	}

	// Blank line between turns gives the bubbles room to breathe; \n after
	// the last block prevents the viewport from clipping the final line.
	return strings.Join(blocks, "\n\n") + "\n"
}

func (m model) renderTurn(role ai.Role, content string, steps []toolStep) string {
	switch role {
	case ai.RoleUser:
		return m.renderUserBubble(content)
	case ai.RoleAssistant:
		return m.renderAssistantBubble(content, steps)
	default:
		// Unknown roles (e.g. system) render with a plain label, no bubble.
		return string(role) + "\n" + content
	}
}

func (m model) renderUserBubble(content string) string {
	body := userBodyStyle.Render(content)
	inner := userLabelStyle.Render("you") + "\n" + body
	return prefixLines(inner, userBarStyle.Render("┃"))
}

func (m model) renderAssistantBubble(content string, steps []toolStep) string {
	body := content
	if m.renderer != nil && content != "" {
		if rendered, err := m.renderer.Render(content); err == nil {
			body = strings.TrimRight(rendered, "\n")
		}
	}

	parts := []string{assistantLabelStyle.Render("bite")}
	if len(steps) > 0 {
		parts = append(parts, m.renderSteps(steps))
	}
	if body != "" {
		parts = append(parts, body)
	}
	return prefixLines(strings.Join(parts, "\n"), assistantBarStyle.Render("┃"))
}

// prefixLines puts `prefix ` in front of every line of s. We do this instead
// of wrapping in lipgloss.Border because the body may contain glamour-
// rendered ANSI; lipgloss re-flow on pre-styled content leaks raw control
// chars (\x01, \x02…) on some terminals.
func prefixLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = prefix + " " + line
	}
	return strings.Join(lines, "\n")
}

func (m model) renderSteps(steps []toolStep) string {
	lines := make([]string, 0, len(steps))
	for _, s := range steps {
		lines = append(lines, m.renderStep(s))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderStep(s toolStep) string {
	args := summarize(s.args, 80)
	if !s.finished {
		head := toolStepRunningStyle.Render("↻ calling " + s.name)
		if args != "" {
			head += toolStepResultStyle.Render(" · " + args)
		}
		return head
	}
	head := toolStepDoneStyle.Render("↳ " + s.name)
	if args != "" {
		head += toolStepResultStyle.Render(" · " + args)
	}
	if result := summarize(s.result, 120); result != "" {
		head += "\n  " + toolStepResultStyle.Render("→ "+result)
	}
	return head
}

// summarize collapses whitespace and truncates s (counted in runes) for
// inline display in the transcript. JSON args and tool results are often
// multi-line; the goal here is a single, readable hint — not a faithful echo.
func summarize(s string, max int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.Join(strings.Fields(s), " ")
	rs := []rune(s)
	if len(rs) > max {
		return string(rs[:max-1]) + "…"
	}
	return s
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
		case ev.ToolStep != nil:
			return streamStepMsg{step: *ev.ToolStep}
		default:
			return streamDeltaMsg{delta: ev.Delta}
		}
	}
}

// applyStep folds a ToolStep event into m.pendingSteps. The "started" event
// appends a new entry; the matching "finished" event (same ID) updates the
// existing one with the result, so the UI shows one row per call rather than
// stacking duplicates.
func (m *model) applyStep(s ai.ToolStep) {
	for i := range m.pendingSteps {
		if m.pendingSteps[i].id == s.ID {
			m.pendingSteps[i].result = s.Result
			m.pendingSteps[i].finished = s.Finished
			return
		}
	}
	m.pendingSteps = append(m.pendingSteps, toolStep{
		id:       s.ID,
		name:     s.Name,
		args:     s.Arguments,
		result:   s.Result,
		finished: s.Finished,
	})
}

func welcomeText() string {
	return strings.Join([]string{
		assistantStyle.Render("bite"),
		"  your terminal nutritionist. log a meal, ask about calories, or just chat.",
		"  Enter to send · /help for slash commands · Ctrl+C to quit",
	}, "\n")
}
