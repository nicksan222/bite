package tui_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/tui"
)

// ─── mocks ────────────────────────────────────────────────────────────────────

type mockStreamer struct {
	resp string
	err  error
}

func (m *mockStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan ai.StreamEvent, 2)
	if m.resp != "" {
		ch <- ai.StreamEvent{Delta: m.resp}
	}
	ch <- ai.StreamEvent{Done: true, Final: m.resp}
	close(ch)
	return ch, nil
}

type mockPersister struct {
	userMsgs      []string
	assistantMsgs []string
	appendErr     error
}

func (m *mockPersister) AppendUser(_ context.Context, content string) error {
	m.userMsgs = append(m.userMsgs, content)
	return m.appendErr
}

func (m *mockPersister) AppendAssistant(_ context.Context, content string) error {
	m.assistantMsgs = append(m.assistantMsgs, content)
	return m.appendErr
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// runUpdate drives a tea.Program one step at a time by calling Update
// on the raw model. Since bubbletea models are not directly exported,
// we exercise via the program's run cycle with the testing program approach.
func newTestProgram(client ai.Streamer, store tui.Persister) *tea.Program {
	return tui.New(context.Background(), client, store, nil, nil)
}

// ─── tests ────────────────────────────────────────────────────────────────────

func TestNew_createsProgram(t *testing.T) {
	p := newTestProgram(&mockStreamer{}, &mockPersister{})
	assert.NotNil(t, p)
}

func TestNew_withHistory(t *testing.T) {
	history := []ai.Message{
		{Role: ai.RoleUser, Content: "hello"},
		{Role: ai.RoleAssistant, Content: "hi"},
	}
	p := tui.New(context.Background(), &mockStreamer{}, &mockPersister{}, history, nil)
	assert.NotNil(t, p)
}

func TestNew_nilStore(t *testing.T) {
	// Should not panic with nil store.
	p := tui.New(context.Background(), &mockStreamer{}, nil, nil, nil)
	assert.NotNil(t, p)
}

func TestMockStreamer_interfaceSatisfied(t *testing.T) {
	var _ ai.Streamer = (*mockStreamer)(nil)
}

func TestMockPersister_interfaceSatisfied(t *testing.T) {
	var _ tui.Persister = (*mockPersister)(nil)
}

func TestNew_acceptsSlashHandler(t *testing.T) {
	p := tui.New(context.Background(), &mockStreamer{}, &mockPersister{}, nil, nil,
		tui.WithSlashHandler(func(_ context.Context, _ string) (string, error) {
			return "ok", nil
		}))
	assert.NotNil(t, p)
}

func TestNew_slashHandlerOptional(t *testing.T) {
	// Without WithSlashHandler, New still produces a valid program — the
	// chat just won't intercept "/cmd" lines (they go to the model).
	require.NotNil(t, tui.New(context.Background(), &mockStreamer{}, &mockPersister{}, nil, nil))
}

func TestMockStreamer_returnsError(t *testing.T) {
	s := &mockStreamer{err: errors.New("no connection")}
	ch, err := s.Stream(context.Background(), nil)
	require.Error(t, err)
	assert.Nil(t, ch)
}

func TestMockStreamer_deliversEvents(t *testing.T) {
	s := &mockStreamer{resp: "hello world"}
	ch, err := s.Stream(context.Background(), nil)
	require.NoError(t, err)

	var got strings.Builder
	for ev := range ch {
		if ev.Done {
			break
		}
		got.WriteString(ev.Delta)
	}
	assert.Equal(t, "hello world", got.String())
}
