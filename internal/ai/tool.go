package ai

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Tool describes a function the model can invoke during a Stream call.
// Execute receives the raw JSON arguments string and returns a plain-text result.
// Errors from Execute are forwarded to the model as a tool-result message so the
// model can react gracefully rather than hard-failing the conversation.
type Tool struct {
	Name        string
	Description string
	// Parameters describes the JSON Schema of the arguments object.
	// Nil means the tool takes no parameters.
	Parameters map[string]*schema.ParameterInfo
	Execute    func(ctx context.Context, args string) (string, error)
}

func (t Tool) toToolInfo() *schema.ToolInfo {
	info := &schema.ToolInfo{
		Name: t.Name,
		Desc: t.Description,
	}
	if len(t.Parameters) > 0 {
		info.ParamsOneOf = schema.NewParamsOneOfByParams(t.Parameters)
	}
	return info
}

// WithTools binds tools to a single Stream call. The model may request one or
// more tool invocations; Stream executes them transparently and continues until
// the model emits a plain text response.
func WithTools(tools []Tool) StreamOption {
	return func(o *streamOpts) { o.tools = tools }
}

// maxToolRounds caps the tool-calling loop so a model that never stops
// requesting tools does not spin forever.
const maxToolRounds = 20

// pumpWithTools runs the multi-turn tool-calling loop. It streams from the
// model, executes any requested tool calls, and loops until the model produces
// a plain-text response, forwarding Delta events on every turn.
func pumpWithTools(ctx context.Context, m model.ToolCallingChatModel, msgs []*schema.Message, tools map[string]Tool, out chan<- StreamEvent) {
	defer close(out)

	for round := 0; round < maxToolRounds; round++ {
		reader, err := m.Stream(ctx, msgs)
		if err != nil {
			sendErr(ctx, out, fmt.Errorf("model stream: %w", err))
			return
		}

		final, ok := drainReader(ctx, reader, out)
		if !ok {
			return
		}

		if len(final.ToolCalls) == 0 {
			select {
			case out <- StreamEvent{Done: true, Final: final.Content}:
			case <-ctx.Done():
			}
			return
		}

		msgs = append(msgs, final)
		for _, tc := range final.ToolCalls {
			result := executeTool(ctx, tools, tc)
			msgs = append(msgs, &schema.Message{
				Role:       schema.Tool,
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}
	sendErr(ctx, out, fmt.Errorf("tool loop exceeded %d rounds", maxToolRounds))
}

// drainReader collects all chunks from reader, emits Delta events, and returns
// the concatenated final message. Returns (nil, false) if the context is
// cancelled or an error event is sent.
func drainReader(ctx context.Context, reader *schema.StreamReader[*schema.Message], out chan<- StreamEvent) (*schema.Message, bool) {
	defer reader.Close()

	var chunks []*schema.Message
	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			sendErr(ctx, out, err)
			return nil, false
		}
		chunks = append(chunks, chunk)
		if chunk.Content != "" {
			select {
			case out <- StreamEvent{Delta: chunk.Content}:
			case <-ctx.Done():
				return nil, false
			}
		}
	}

	final, err := schema.ConcatMessages(chunks)
	if err != nil {
		sendErr(ctx, out, fmt.Errorf("concat messages: %w", err))
		return nil, false
	}
	return final, true
}

// executeTool runs one tool call and returns its result string.
// Unknown tools and execution errors are returned as descriptive strings so the
// model can handle them gracefully.
func executeTool(ctx context.Context, tools map[string]Tool, tc schema.ToolCall) string {
	t, ok := tools[tc.Function.Name]
	if !ok {
		return fmt.Sprintf("error: unknown tool %q", tc.Function.Name)
	}
	result, err := t.Execute(ctx, tc.Function.Arguments)
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}

func sendErr(ctx context.Context, out chan<- StreamEvent, err error) {
	select {
	case out <- StreamEvent{Err: err}:
	case <-ctx.Done():
	}
}
