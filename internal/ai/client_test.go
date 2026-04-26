package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockModel is a simple ToolCallingChatModel used for pump and Stream tests.
type mockModel struct {
	chunks []*schema.Message
	err    error
}

func (m *mockModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, errors.New("Generate not used in tests")
}

func (m *mockModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.err != nil {
		return nil, m.err
	}
	sr, sw := schema.Pipe[*schema.Message](len(m.chunks) + 1)
	for _, chunk := range m.chunks {
		sw.Send(chunk, nil)
	}
	sw.Close()
	return sr, nil
}

func (m *mockModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

var _ model.ToolCallingChatModel = (*mockModel)(nil)

// ─── NewClient ────────────────────────────────────────────────────────────────

func TestNewClientValidates(t *testing.T) {
	ctx := context.Background()
	_, err := NewClient(ctx, ClientConfig{Model: "m"})
	assert.Error(t, err, "missing API key should error")

	_, err = NewClient(ctx, ClientConfig{APIKey: "k"})
	assert.Error(t, err, "missing model should error")
}

func TestNewClient_success(t *testing.T) {
	c, err := NewClient(context.Background(), ClientConfig{
		APIKey: "sk-ant-test-fake",
		Model:  "claude-haiku-4-5",
	})
	require.NoError(t, err)
	assert.NotNil(t, c)
}

func TestStreamNilClientReturnsError(t *testing.T) {
	var c *Client
	_, err := c.Stream(context.Background(), nil)
	assert.Error(t, err)
}

func TestStreamerInterfaceSatisfied(t *testing.T) {
	var _ Streamer = (*Client)(nil)
}

// ─── buildSchemaMessages ─────────────────────────────────────────────────────

func TestBuildSchemaMessages_PrependsSystemPrompt(t *testing.T) {
	got, err := buildSchemaMessages([]Message{{Role: RoleUser, Content: "hi"}}, "be terse")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, schema.System, got[0].Role)
	assert.Equal(t, "be terse", got[0].Content)
}

func TestBuildSchemaMessages_NoDoubleSystem(t *testing.T) {
	got, err := buildSchemaMessages([]Message{
		{Role: RoleSystem, Content: "you are bite"},
		{Role: RoleUser, Content: "hi"},
	}, "be terse")
	require.NoError(t, err)
	require.Len(t, got, 2, "system already present, no prepend")
	assert.Equal(t, "you are bite", got[0].Content)
}

func TestBuildSchemaMessages_EmptySystemPromptNoPrepend(t *testing.T) {
	got, err := buildSchemaMessages([]Message{{Role: RoleUser, Content: "hi"}}, "")
	require.NoError(t, err)
	assert.Len(t, got, 1, "no system prompt injected")
}

// ─── WithSystemPrompt ─────────────────────────────────────────────────────────

func TestWithSystemPrompt_setsOverride(t *testing.T) {
	o := &streamOpts{}
	WithSystemPrompt("custom")(o)
	require.NotNil(t, o.systemPromptOverride)
	assert.Equal(t, "custom", *o.systemPromptOverride)
}

func TestWithSystemPrompt_emptyDisablesInjection(t *testing.T) {
	o := &streamOpts{}
	WithSystemPrompt("")(o)
	require.NotNil(t, o.systemPromptOverride, "pointer must be non-nil even for empty string")
	assert.Equal(t, "", *o.systemPromptOverride)
}

// ─── pump ─────────────────────────────────────────────────────────────────────

func TestPump_deltaAndDone(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	sw.Send(&schema.Message{Content: "hello "}, nil)
	sw.Send(&schema.Message{Content: "world"}, nil)
	sw.Close()

	out := make(chan StreamEvent, 8)
	pump(context.Background(), sr, out)

	var events []StreamEvent
	for ev := range out {
		events = append(events, ev)
	}
	require.GreaterOrEqual(t, len(events), 3, "expected at least 2 deltas + done")
	last := events[len(events)-1]
	assert.True(t, last.Done)
	assert.Equal(t, "hello world", last.Final)
}

func TestPump_emptyChunk_noEvent(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](2)
	sw.Send(&schema.Message{Content: ""}, nil)
	sw.Close()

	out := make(chan StreamEvent, 4)
	pump(context.Background(), sr, out)

	var events []StreamEvent
	for ev := range out {
		events = append(events, ev)
	}
	require.Len(t, events, 1, "only Done event expected for empty chunk")
	assert.True(t, events[0].Done)
}

func TestPump_readerError(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](2)
	sw.Send(nil, errors.New("network blip"))
	sw.Close()

	out := make(chan StreamEvent, 4)
	pump(context.Background(), sr, out)

	var events []StreamEvent
	for ev := range out {
		events = append(events, ev)
	}
	require.Len(t, events, 1)
	assert.Error(t, events[0].Err)
}

func TestPump_contextCancelled_buffered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sr, sw := schema.Pipe[*schema.Message](2)
	sw.Send(&schema.Message{Content: "data"}, nil)
	sw.Close()

	out := make(chan StreamEvent, 4)
	pump(ctx, sr, out)
	for range out {
	}
}

func TestPump_contextCancelled_unbuffered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sr, sw := schema.Pipe[*schema.Message](2)
	sw.Send(&schema.Message{Content: "data"}, nil)
	sw.Close()

	out := make(chan StreamEvent)
	done := make(chan struct{})
	go func() {
		defer close(done)
		pump(ctx, sr, out)
	}()
	<-done
}

func TestPump_concatMessagesError(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	sw.Send(&schema.Message{Role: schema.User, Content: "hello"}, nil)
	sw.Send(&schema.Message{Role: schema.Assistant, Content: "world"}, nil)
	sw.Close()

	out := make(chan StreamEvent, 8)
	pump(context.Background(), sr, out)

	var gotErr bool
	for ev := range out {
		if ev.Err != nil {
			gotErr = true
		}
	}
	assert.True(t, gotErr, "expected Err event from ConcatMessages role mismatch")
}

// ─── Stream (no tools) ────────────────────────────────────────────────────────

func TestStream_withMockModel_deliversEvents(t *testing.T) {
	c := &Client{model: &mockModel{chunks: []*schema.Message{
		{Content: "rice"},
		{Content: " bowl"},
	}}}

	ch, err := c.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	require.NoError(t, err)

	var full string
	for ev := range ch {
		require.NoError(t, ev.Err)
		if ev.Done {
			full = ev.Final
			break
		}
		full += ev.Delta
	}
	assert.NotEmpty(t, full)
}

func TestStream_withSystemPromptOverride(t *testing.T) {
	c := &Client{
		model:        &mockModel{chunks: []*schema.Message{{Content: "ok"}}},
		systemPrompt: "default prompt",
	}
	ch, err := c.Stream(
		context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}},
		WithSystemPrompt("override prompt"),
	)
	require.NoError(t, err)
	for range ch {
	}
}

func TestStream_modelStreamError(t *testing.T) {
	c := &Client{model: &mockModel{err: errors.New("model unavailable")}}
	_, err := c.Stream(context.Background(), []Message{{Role: RoleUser, Content: "hi"}})
	require.Error(t, err)
	assert.ErrorContains(t, err, "model unavailable")
}

func TestStream_buildSchemaMessagesError(t *testing.T) {
	c := &Client{model: &mockModel{}}
	_, err := c.Stream(context.Background(), []Message{{
		Role:        RoleUser,
		Attachments: []Attachment{{URL: "x.mp3", MimeType: "audio/mpeg"}},
	}})
	assert.Error(t, err, "unsupported MIME type should fail")
}
