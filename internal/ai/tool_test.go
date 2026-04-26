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

// ─── multi-call mock ──────────────────────────────────────────────────────────

// callableModel returns a different response on each successive Stream call,
// enabling multi-turn tool-loop testing without a real provider.
type callableModel struct {
	responses []callResponse
	idx       int
}

type callResponse struct {
	chunks    []*schema.Message
	streamErr error // error returned by Stream() before any reads
	readerErr error // error injected into the reader itself (via Pipe.Send)
}

func (m *callableModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return nil, errors.New("Generate not used")
}

func (m *callableModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m.idx >= len(m.responses) {
		return nil, errors.New("callableModel: unexpected extra Stream call")
	}
	resp := m.responses[m.idx]
	m.idx++
	if resp.streamErr != nil {
		return nil, resp.streamErr
	}
	sr, sw := schema.Pipe[*schema.Message](len(resp.chunks) + 2)
	for _, c := range resp.chunks {
		sw.Send(c, nil)
	}
	if resp.readerErr != nil {
		sw.Send(nil, resp.readerErr)
	}
	sw.Close()
	return sr, nil
}

func (m *callableModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

var _ model.ToolCallingChatModel = (*callableModel)(nil)

// ─── Tool.toToolInfo ─────────────────────────────────────────────────────────

func TestTool_toToolInfo_withParameters(t *testing.T) {
	tool := Tool{
		Name:        "log_meal",
		Description: "logs a meal",
		Parameters: map[string]*schema.ParameterInfo{
			"food": {Type: schema.String, Desc: "food name", Required: true},
		},
	}
	info := tool.toToolInfo()
	assert.Equal(t, "log_meal", info.Name)
	assert.Equal(t, "logs a meal", info.Desc)
	assert.NotNil(t, info.ParamsOneOf)
}

func TestTool_toToolInfo_noParameters(t *testing.T) {
	tool := Tool{Name: "get_time", Description: "current time"}
	info := tool.toToolInfo()
	assert.Nil(t, info.ParamsOneOf, "expected nil ParamsOneOf for tool with no parameters")
}

// ─── WithTools StreamOption ───────────────────────────────────────────────────

func TestWithTools_setsOption(t *testing.T) {
	tools := []Tool{{Name: "t1"}}
	o := &streamOpts{}
	WithTools(tools)(o)
	require.Len(t, o.tools, 1)
	assert.Equal(t, "t1", o.tools[0].Name)
}

func TestWithTools_empty_setsEmptySlice(t *testing.T) {
	o := &streamOpts{}
	WithTools(nil)(o)
	assert.Nil(t, o.tools)
}

// ─── executeTool ─────────────────────────────────────────────────────────────

func TestExecuteTool_success(t *testing.T) {
	tools := map[string]Tool{
		"echo": {Execute: func(_ context.Context, args string) (string, error) {
			return "got: " + args, nil
		}},
	}
	got := executeTool(context.Background(), tools, schema.ToolCall{
		Function: schema.FunctionCall{Name: "echo", Arguments: `{"msg":"hi"}`},
	})
	assert.Equal(t, `got: {"msg":"hi"}`, got)
}

func TestExecuteTool_executeError(t *testing.T) {
	tools := map[string]Tool{
		"fail": {Execute: func(_ context.Context, _ string) (string, error) {
			return "", errors.New("db error")
		}},
	}
	got := executeTool(context.Background(), tools, schema.ToolCall{
		Function: schema.FunctionCall{Name: "fail"},
	})
	assert.Contains(t, got, "db error")
}

func TestExecuteTool_unknownTool(t *testing.T) {
	got := executeTool(context.Background(), map[string]Tool{}, schema.ToolCall{
		Function: schema.FunctionCall{Name: "no_such_tool"},
	})
	assert.Contains(t, got, "unknown tool")
}

// ─── pumpWithTools ────────────────────────────────────────────────────────────

func TestPumpWithTools_noToolCalls_directText(t *testing.T) {
	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{Content: "eat more vegetables"}}},
	}}
	out := make(chan StreamEvent, 16)
	pumpWithTools(context.Background(), m, nil, nil, out)

	var final string
	for ev := range out {
		require.NoError(t, ev.Err)
		if ev.Done {
			final = ev.Final
		}
	}
	assert.Equal(t, "eat more vegetables", final)
}

func TestPumpWithTools_emitsToolStepEvents(t *testing.T) {
	tools := map[string]Tool{
		"log_meal": {Execute: func(_ context.Context, args string) (string, error) {
			return "logged: " + args, nil
		}},
	}
	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID: "abc123", Type: "function",
				Function: schema.FunctionCall{Name: "log_meal", Arguments: `{"food":"pasta"}`},
			}},
		}}},
		{chunks: []*schema.Message{{Content: "ok"}}},
	}}

	out := make(chan StreamEvent, 16)
	pumpWithTools(context.Background(), m, nil, tools, out)

	var steps []ToolStep
	for ev := range out {
		if ev.ToolStep != nil {
			steps = append(steps, *ev.ToolStep)
		}
	}
	require.Len(t, steps, 2, "expected one start + one finish event")
	assert.Equal(t, "abc123", steps[0].ID)
	assert.Equal(t, "log_meal", steps[0].Name)
	assert.False(t, steps[0].Finished, "first step should be the start event")
	assert.Empty(t, steps[0].Result)
	assert.True(t, steps[1].Finished, "second step should be the finish event")
	assert.Equal(t, `logged: {"food":"pasta"}`, steps[1].Result)
	assert.Equal(t, steps[0].ID, steps[1].ID, "matching ID lets UIs update in place")
}

func TestPumpWithTools_singleToolCall_thenText(t *testing.T) {
	callCount := 0
	tools := map[string]Tool{
		"log_meal": {Execute: func(_ context.Context, args string) (string, error) {
			callCount++
			return "meal logged: " + args, nil
		}},
	}

	m := &callableModel{responses: []callResponse{
		// Round 1: model requests tool call.
		{chunks: []*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID: "call_1", Type: "function",
				Function: schema.FunctionCall{Name: "log_meal", Arguments: `{"food":"pasta"}`},
			}},
		}}},
		// Round 2: model delivers plain response.
		{chunks: []*schema.Message{{Content: "logged successfully"}}},
	}}

	out := make(chan StreamEvent, 16)
	pumpWithTools(context.Background(), m, nil, tools, out)

	assert.Equal(t, 1, callCount, "tool executed %d times, want 1", callCount)

	var final string
	for ev := range out {
		require.NoError(t, ev.Err)
		if ev.Done {
			final = ev.Final
		}
	}
	assert.Equal(t, "logged successfully", final)
}

func TestPumpWithTools_multipleToolCallRounds(t *testing.T) {
	calls := 0
	tools := map[string]Tool{
		"t": {Execute: func(_ context.Context, _ string) (string, error) {
			calls++
			return "ok", nil
		}},
	}
	toolCall := schema.ToolCall{ID: "c1", Type: "function", Function: schema.FunctionCall{Name: "t"}}

	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{toolCall}}}},
		{chunks: []*schema.Message{{Role: schema.Assistant, ToolCalls: []schema.ToolCall{toolCall}}}},
		{chunks: []*schema.Message{{Content: "all done"}}},
	}}

	out := make(chan StreamEvent, 32)
	pumpWithTools(context.Background(), m, nil, tools, out)

	assert.Equal(t, 2, calls, "tool called %d times, want 2", calls)
	var final string
	for ev := range out {
		if ev.Done {
			final = ev.Final
		}
	}
	assert.Equal(t, "all done", final)
}

func TestPumpWithTools_streamError_firstRound(t *testing.T) {
	m := &callableModel{responses: []callResponse{
		{streamErr: errors.New("api down")},
	}}
	out := make(chan StreamEvent, 8)
	pumpWithTools(context.Background(), m, nil, nil, out)

	var gotErr bool
	for ev := range out {
		if ev.Err != nil {
			gotErr = true
		}
	}
	assert.True(t, gotErr, "expected error event when first Stream call fails")
}

func TestPumpWithTools_streamError_afterToolExecution(t *testing.T) {
	tools := map[string]Tool{
		"t": {Execute: func(_ context.Context, _ string) (string, error) { return "ok", nil }},
	}
	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID: "c1", Type: "function",
				Function: schema.FunctionCall{Name: "t"},
			}},
		}}},
		{streamErr: errors.New("second call failed")},
	}}

	out := make(chan StreamEvent, 16)
	pumpWithTools(context.Background(), m, nil, tools, out)

	var gotErr bool
	for ev := range out {
		if ev.Err != nil {
			gotErr = true
		}
	}
	assert.True(t, gotErr, "expected error event on second Stream call failure")
}

func TestPumpWithTools_unknownTool_returnsErrorString(t *testing.T) {
	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{
			Role: schema.Assistant,
			ToolCalls: []schema.ToolCall{{
				ID: "c1", Type: "function",
				Function: schema.FunctionCall{Name: "no_such_tool"},
			}},
		}}},
		{chunks: []*schema.Message{{Content: "handled it"}}},
	}}

	out := make(chan StreamEvent, 16)
	pumpWithTools(context.Background(), m, nil, map[string]Tool{}, out)

	var final string
	for ev := range out {
		require.NoError(t, ev.Err)
		if ev.Done {
			final = ev.Final
		}
	}
	assert.Equal(t, "handled it", final)
}

func TestPumpWithTools_drainError_afterToolExecution(t *testing.T) {
	// Round 0: model returns a tool call (drainReader returns ok=true).
	// Round 1: Stream succeeds but the reader itself errors (drainReader returns ok=false).
	// This covers the if !ok { return } branch at the start of each loop body.
	tools := map[string]Tool{
		"t": {Execute: func(_ context.Context, _ string) (string, error) { return "ok", nil }},
	}
	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{
			Role:      schema.Assistant,
			ToolCalls: []schema.ToolCall{{ID: "c1", Type: "function", Function: schema.FunctionCall{Name: "t"}}},
		}}},
		{readerErr: errors.New("reader blew up")},
	}}

	out := make(chan StreamEvent, 16)
	pumpWithTools(context.Background(), m, nil, tools, out)

	var gotErr bool
	for ev := range out {
		if ev.Err != nil {
			gotErr = true
		}
	}
	assert.True(t, gotErr, "expected error event when second-round reader fails")
}

func TestPumpWithTools_maxRoundsExceeded(t *testing.T) {
	// Build a model that always returns a tool call, triggering the loop cap.
	toolCall := schema.ToolCall{ID: "c1", Type: "function", Function: schema.FunctionCall{Name: "t"}}
	responses := make([]callResponse, maxToolRounds)
	for i := range responses {
		responses[i] = callResponse{chunks: []*schema.Message{{
			Role:      schema.Assistant,
			ToolCalls: []schema.ToolCall{toolCall},
		}}}
	}
	m := &callableModel{responses: responses}
	tools := map[string]Tool{
		"t": {Execute: func(_ context.Context, _ string) (string, error) { return "ok", nil }},
	}

	out := make(chan StreamEvent, maxToolRounds*4)
	pumpWithTools(context.Background(), m, nil, tools, out)

	var gotErr bool
	for ev := range out {
		if ev.Err != nil && assert.ErrorContains(t, ev.Err, "exceeded") {
			gotErr = true
		}
	}
	assert.True(t, gotErr, "expected 'exceeded' error after maxToolRounds")
}

func TestPumpWithTools_contextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m := &callableModel{responses: []callResponse{
		{chunks: []*schema.Message{{Content: "response"}}},
	}}
	out := make(chan StreamEvent, 8)
	pumpWithTools(ctx, m, nil, nil, out)
	for range out {
	}
}

// ─── Stream + tools integration ───────────────────────────────────────────────

func TestStream_withTools_noToolCalls(t *testing.T) {
	c := &Client{
		model: &callableModel{responses: []callResponse{
			{chunks: []*schema.Message{{Content: "plain answer"}}},
		}},
	}
	ch, err := c.Stream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}},
		WithTools([]Tool{{Name: "unused"}}),
	)
	require.NoError(t, err)
	var final string
	for ev := range ch {
		require.NoError(t, ev.Err)
		if ev.Done {
			final = ev.Final
		}
	}
	assert.Equal(t, "plain answer", final)
}

func TestStream_withTools_executesTool(t *testing.T) {
	toolCalled := false
	tool := Tool{
		Name:    "ping",
		Execute: func(_ context.Context, _ string) (string, error) { toolCalled = true; return "pong", nil },
	}
	c := &Client{
		model: &callableModel{responses: []callResponse{
			{chunks: []*schema.Message{{
				Role: schema.Assistant,
				ToolCalls: []schema.ToolCall{{
					ID: "c1", Type: "function",
					Function: schema.FunctionCall{Name: "ping"},
				}},
			}}},
			{chunks: []*schema.Message{{Content: "pong received"}}},
		}},
	}

	ch, err := c.Stream(context.Background(),
		[]Message{{Role: RoleUser, Content: "ping"}},
		WithTools([]Tool{tool}),
	)
	require.NoError(t, err)

	var final string
	for ev := range ch {
		require.NoError(t, ev.Err)
		if ev.Done {
			final = ev.Final
		}
	}
	assert.True(t, toolCalled, "expected tool to be called")
	assert.Equal(t, "pong received", final)
}

// ─── Stream + WithTools error ─────────────────────────────────────────────────

// errBindModel returns an error from WithTools.
type errBindModel struct {
	*callableModel
}

func (m *errBindModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return nil, errors.New("bind tools failed")
}

func TestStream_withTools_bindError(t *testing.T) {
	c := &Client{
		model: &errBindModel{callableModel: &callableModel{}},
	}
	_, err := c.Stream(context.Background(),
		[]Message{{Role: RoleUser, Content: "hi"}},
		WithTools([]Tool{{Name: "t"}}),
	)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind tools")
}

// ─── drainReader ──────────────────────────────────────────────────────────────

func TestDrainReader_readerError(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](2)
	sw.Send(nil, errors.New("stream broke"))
	sw.Close()

	out := make(chan StreamEvent, 4)
	_, ok := drainReader(context.Background(), sr, out)
	close(out)

	assert.False(t, ok, "expected ok=false on reader error")
	var gotErr bool
	for ev := range out {
		if ev.Err != nil {
			gotErr = true
		}
	}
	assert.True(t, gotErr, "expected error event")
}

func TestDrainReader_emitsDeltas(t *testing.T) {
	sr, sw := schema.Pipe[*schema.Message](4)
	sw.Send(&schema.Message{Content: "hello"}, nil)
	sw.Send(&schema.Message{Content: " world"}, nil)
	sw.Close()

	out := make(chan StreamEvent, 8)
	final, ok := drainReader(context.Background(), sr, out)
	close(out)

	require.True(t, ok, "expected ok=true")
	require.NotNil(t, final)
	assert.Equal(t, "hello world", final.Content)
	var deltas int
	for ev := range out {
		if ev.Delta != "" {
			deltas++
		}
	}
	assert.Equal(t, 2, deltas, "expected 2 delta events, got %d", deltas)
}

func TestDrainReader_contextCancelled_unbuffered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	sr, sw := schema.Pipe[*schema.Message](2)
	sw.Send(&schema.Message{Content: "data"}, nil)
	sw.Close()

	out := make(chan StreamEvent) // unbuffered
	done := make(chan struct{})
	go func() {
		defer close(done)
		drainReader(ctx, sr, out) //nolint:errcheck
	}()
	<-done
}
