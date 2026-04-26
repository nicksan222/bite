package tui

// Internal (whitebox) tests for the model's Update/View logic.

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// ─── mocks ────────────────────────────────────────────────────────────────────

type testStreamer struct {
	resp string
	err  error
}

func (s *testStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan ai.StreamEvent, 4)
	if s.resp != "" {
		ch <- ai.StreamEvent{Delta: s.resp}
	}
	ch <- ai.StreamEvent{Done: true, Final: s.resp}
	close(ch)
	return ch, nil
}

type testPersister struct {
	userMsgs      []string
	assistantMsgs []string
	err           error
}

func (p *testPersister) AppendUser(_ context.Context, content string) error {
	p.userMsgs = append(p.userMsgs, content)
	return p.err
}

func (p *testPersister) AppendAssistant(_ context.Context, content string) error {
	p.assistantMsgs = append(p.assistantMsgs, content)
	return p.err
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newModel(client ai.Streamer, store Persister, history []ai.Message) model {
	return initialModel(context.Background(), client, store, history, nil)
}

func asModel(t *testing.T, tm tea.Model) model {
	t.Helper()
	m, ok := tm.(model)
	require.True(t, ok, "Update returned unexpected type %T", tm)
	return m
}

func update(t *testing.T, m model, msg tea.Msg) (model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return asModel(t, next), cmd
}

// ─── Update tests ─────────────────────────────────────────────────────────────

func TestUpdate_windowSize(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	got, _ := update(t, m, tea.WindowSizeMsg{Width: 120, Height: 50})
	assert.Equal(t, 120, got.width)
	assert.Equal(t, 50, got.height)
}

func TestUpdate_streamDelta_accumulatesPending(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m.streaming = true

	got, _ := update(t, m, streamDeltaMsg{delta: "hello "})
	assert.Equal(t, "hello ", got.pending)

	got2, _ := update(t, got, streamDeltaMsg{delta: "world"})
	assert.Equal(t, "hello world", got2.pending)
}

func TestUpdate_streamDone_appendsToHistory(t *testing.T) {
	p := &testPersister{}
	m := newModel(&testStreamer{}, p, nil)
	m.streaming = true
	m.pending = "partials so far"

	got, _ := update(t, m, streamDoneMsg{full: "final answer"})

	assert.False(t, got.streaming, "streaming should be false after Done")
	assert.Empty(t, got.pending, "pending should be cleared")
	require.Len(t, got.history, 1)
	assert.Equal(t, "final answer", got.history[0].Content)
	assert.Equal(t, ai.RoleAssistant, got.history[0].Role)
	require.Len(t, p.assistantMsgs, 1)
	assert.Equal(t, "final answer", p.assistantMsgs[0])
}

func TestUpdate_streamDone_usePendingWhenFullEmpty(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m.streaming = true
	m.pending = "accumulated"

	got, _ := update(t, m, streamDoneMsg{full: ""})
	require.Len(t, got.history, 1)
	assert.Equal(t, "accumulated", got.history[0].Content)
}

func TestUpdate_streamErr_setsError(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m.streaming = true
	m.pending = "partial"

	got, _ := update(t, m, streamErrMsg{err: errors.New("connection lost")})

	assert.False(t, got.streaming, "streaming should be false after error")
	assert.Empty(t, got.pending, "pending should be cleared on error")
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "connection lost")
}

func TestUpdate_send_startsSend(t *testing.T) {
	p := &testPersister{}
	m := newModel(&testStreamer{resp: "hi"}, p, nil)
	next, _ := update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})

	rawGot, cmd := next.send("what should I eat?")
	got := asModel(t, rawGot)

	assert.True(t, got.streaming, "expected streaming=true after send")
	require.Len(t, got.history, 1)
	assert.Equal(t, "what should I eat?", got.history[0].Content)
	assert.Len(t, p.userMsgs, 1)
	assert.NotNil(t, cmd, "expected a non-nil command (readNext + spinner tick)")
}

func TestUpdate_send_streamError(t *testing.T) {
	m := newModel(&testStreamer{err: errors.New("api error")}, nil, nil)

	rawGot, _ := m.send("hello")
	got := asModel(t, rawGot)

	assert.NotNil(t, got.err, "expected error to be set on stream failure")
	assert.False(t, got.streaming, "streaming should be false when Stream() errors")
}

func TestUpdate_ctrlC_quitsProgram(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	require.NotNil(t, cmd, "expected quit command from ctrl+c")
}

func TestUpdate_enterWhileStreaming_isNoOp(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m.streaming = true

	got, _ := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, got.streaming, "streaming flag should remain true (input ignored while streaming)")
	assert.Len(t, got.history, 0, "no new messages should appear while streaming")
}

func TestUpdate_persistenceError_setsErr(t *testing.T) {
	p := &testPersister{err: errors.New("disk full")}
	m := newModel(&testStreamer{}, p, nil)
	m.streaming = true

	got, _ := update(t, m, streamDoneMsg{full: "some response"})
	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "disk full")
}

func TestUpdate_ctrlD_quitsProgram(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	require.NotNil(t, cmd, "expected quit command from ctrl+d")
}

func TestUpdate_emptyEnter_isNoOp(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})

	// Don't type anything — input is blank, enter should do nothing.
	got, _ := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Len(t, got.history, 0, "empty input should not add a history entry")
	assert.False(t, got.streaming, "streaming should not start on empty enter")
}

func TestUpdate_enterWithText_sends(t *testing.T) {
	m := newModel(&testStreamer{resp: "great choice!"}, &testPersister{}, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})

	// Type "hi" into the textarea by sending rune events.
	for _, r := range "hi" {
		m, _ = update(t, m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	// Press Enter — should send the text and start streaming.
	got, cmd := update(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, got.streaming, "expected streaming=true after Enter with text")
	assert.NotEmpty(t, got.history, "expected user message in history")
	assert.NotNil(t, cmd, "expected non-nil command after send")
}

func TestUpdate_spinnerTick_whileStreaming(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m.streaming = true

	// TickMsg while streaming should update the spinner without error.
	got, cmd := m.Update(m.spinner.Tick())
	gotM := asModel(t, got)
	assert.True(t, gotM.streaming, "streaming should still be true after spinner tick")
	assert.NotNil(t, cmd, "expected non-nil cmd (next spinner tick) while streaming")
}

func TestUpdate_send_persistUserError(t *testing.T) {
	p := &testPersister{err: errors.New("write fail")}
	m := newModel(&testStreamer{resp: "hi"}, p, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})

	rawGot, _ := m.send("anything")
	got := asModel(t, rawGot)

	require.Error(t, got.err)
	assert.Contains(t, got.err.Error(), "write fail")
	// streaming must NOT start: the user turn isn't persisted, so an
	// assistant reply would create an orphaned DB row.
	assert.False(t, got.streaming, "streaming should not start when AppendUser fails")
}

func TestReadNext_channelClosed(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	ch := make(chan ai.StreamEvent)
	close(ch)
	m.streamCh = ch

	cmd := m.readNext()
	require.NotNil(t, cmd)
	msg := cmd()
	_, ok := msg.(streamDoneMsg)
	assert.True(t, ok, "closed channel should produce streamDoneMsg, got %T", msg)
}

func TestReadNext_errEvent(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Err: errors.New("stream broke")}
	m.streamCh = ch

	msg := m.readNext()()
	e, ok := msg.(streamErrMsg)
	require.True(t, ok, "expected streamErrMsg, got %T", msg)
	assert.NotNil(t, e.err)
}

func TestReadNext_doneEvent(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Done: true, Final: "the end"}
	m.streamCh = ch

	msg := m.readNext()()
	done, ok := msg.(streamDoneMsg)
	require.True(t, ok, "expected streamDoneMsg, got %T", msg)
	assert.Equal(t, "the end", done.full)
}

func TestReadNext_deltaEvent(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	ch := make(chan ai.StreamEvent, 1)
	ch <- ai.StreamEvent{Delta: "chunk"}
	m.streamCh = ch

	msg := m.readNext()()
	delta, ok := msg.(streamDeltaMsg)
	require.True(t, ok, "expected streamDeltaMsg, got %T", msg)
	assert.Equal(t, "chunk", delta.delta)
}

func TestInit_returnsCmd(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	cmd := m.Init()
	require.NotNil(t, cmd, "Init should return a non-nil batch command")
}

func TestLayout_smallDimensions(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	// Should not panic for tiny window
	m.width, m.height = 5, 5
	m.layout()
}

func TestRenderTurn_unknownRole(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	result := m.renderTurn("system", "some system content")
	assert.Contains(t, result, "system")
}

// ─── View tests ───────────────────────────────────────────────────────────────

func TestView_nonEmpty(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	v := m.View()
	assert.NotEmpty(t, v, "View should return non-empty string")
}

func TestView_containsStatusLine(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	v := m.View()
	hasReady := strings.Contains(v, "ready")
	hasBite := strings.Contains(v, "bite")
	assert.True(t, hasReady || hasBite, "expected 'ready' or 'bite' in view, got: %q", v)
}

func TestView_showsErrorState(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m.err = errors.New("something broke")
	v := m.View()
	assert.Contains(t, v, "something broke")
}

func TestView_showsStreamingState(t *testing.T) {
	m := newModel(&testStreamer{}, nil, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	m.streaming = true
	v := m.View()
	assert.Contains(t, v, "thinking")
}

// ─── handleSlash ──────────────────────────────────────────────────────────────

// sendLine drives model.send through the same type-assertion path the
// production tea loop uses. Saves repeating asModel everywhere.
func sendLine(t *testing.T, m model, text string) model {
	t.Helper()
	tm, _ := m.send(text)
	return asModel(t, tm)
}

func TestHandleSlash_appendsHistoryAndPersistsBothSides(t *testing.T) {
	p := &testPersister{}
	m := newModel(&testStreamer{}, p, nil)
	m.slash = func(_ context.Context, line string) (string, error) {
		assert.Equal(t, "/log_meal pasta", line)
		return "meal logged", nil
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	got := sendLine(t, m, "/log_meal pasta")

	require.Len(t, got.history, 2)
	assert.Equal(t, ai.RoleUser, got.history[0].Role)
	assert.Equal(t, "/log_meal pasta", got.history[0].Content)
	assert.Equal(t, ai.RoleAssistant, got.history[1].Role)
	assert.Equal(t, "meal logged", got.history[1].Content)
	assert.Equal(t, []string{"/log_meal pasta"}, p.userMsgs)
	assert.Equal(t, []string{"meal logged"}, p.assistantMsgs)
}

func TestHandleSlash_parseErrorIsInlineNoHistory(t *testing.T) {
	p := &testPersister{}
	m := newModel(&testStreamer{}, p, nil)
	m.slash = func(_ context.Context, _ string) (string, error) {
		return "", errors.New("unknown command: /nope")
	}

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	got := sendLine(t, m, "/nope")

	require.Error(t, got.err)
	assert.Empty(t, got.history, "parse errors must not pollute persisted history")
	assert.Empty(t, p.userMsgs)
	assert.Empty(t, p.assistantMsgs)
}

func TestHandleSlash_persistenceErrorSurfacesButHistoryStays(t *testing.T) {
	p := &testPersister{err: errors.New("disk full")}
	m := newModel(&testStreamer{}, p, nil)
	m.slash = func(_ context.Context, _ string) (string, error) { return "ok", nil }

	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	got := sendLine(t, m, "/cmd")

	require.Error(t, got.err)
	require.Len(t, got.history, 2, "in-memory history still records the slash turn")
}

func TestHandleSlash_unsetSlashHandlerFallsThroughToModel(t *testing.T) {
	// Without WithSlashHandler, "/cmd" lines must go to the model like any
	// other input — no silent dropping.
	s := &testStreamer{resp: "fallback"}
	m := newModel(s, &testPersister{}, nil)
	m, _ = update(t, m, tea.WindowSizeMsg{Width: 80, Height: 30})
	got := sendLine(t, m, "/cmd")

	assert.True(t, got.streaming, "with no slash handler, the line should hit the model")
}
