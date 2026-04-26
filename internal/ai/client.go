package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ClientConfig configures a streaming AI client.
type ClientConfig struct {
	APIKey       string
	Model        string
	MaxTokens    int
	SystemPrompt string
}

// Streamer is the interface for making streaming AI requests.
// *Client implements Streamer.
type Streamer interface {
	Stream(ctx context.Context, history []Message, opts ...StreamOption) (<-chan StreamEvent, error)
}

// compile-time assertion
var _ Streamer = (*Client)(nil)

// Client is bite's high-level entry point into the AI layer.
type Client struct {
	model        model.ToolCallingChatModel
	systemPrompt string
}

func NewClient(ctx context.Context, cfg ClientConfig) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, errors.New("ai: missing API key")
	}
	if cfg.Model == "" {
		return nil, errors.New("ai: missing model id")
	}
	m, err := newClaudeModel(ctx, cfg.APIKey, cfg.Model, cfg.MaxTokens)
	if err != nil {
		return nil, err
	}
	return &Client{model: m, systemPrompt: cfg.SystemPrompt}, nil
}

// StreamEvent is one chunk delivered while a response is being generated.
// Exactly one of Delta, Err, or Done is set.
type StreamEvent struct {
	Delta string
	Err   error
	Done  bool
	Final string
}

// streamOpts is the internal state built up by StreamOption values.
type streamOpts struct {
	systemPromptOverride *string // nil = use Client default
	tools                []Tool
}

// StreamOption tweaks a single Stream call.
type StreamOption func(*streamOpts)

// WithSystemPrompt overrides the Client's configured system prompt for one
// call. Pass "" to disable system-prompt injection entirely (useful when the
// caller wants the model to follow only the user-message instruction, e.g.
// strict-JSON output).
func WithSystemPrompt(s string) StreamOption {
	return func(o *streamOpts) { o.systemPromptOverride = &s }
}

// Stream sends history to the model and returns a channel of incremental
// events. The channel is closed after a Done event (or an Err event).
func (c *Client) Stream(ctx context.Context, history []Message, opts ...StreamOption) (<-chan StreamEvent, error) {
	if c == nil || c.model == nil {
		return nil, errors.New("ai: client is nil")
	}

	o := &streamOpts{}
	for _, opt := range opts {
		opt(o)
	}

	sys := c.systemPrompt
	if o.systemPromptOverride != nil {
		sys = *o.systemPromptOverride
	}

	in, err := buildSchemaMessages(history, sys)
	if err != nil {
		return nil, err
	}

	out := make(chan StreamEvent, 32)

	if len(o.tools) == 0 {
		reader, err := c.model.Stream(ctx, in)
		if err != nil {
			return nil, fmt.Errorf("model stream: %w", err)
		}
		go pump(ctx, reader, out)
		return out, nil
	}

	toolInfos := make([]*schema.ToolInfo, len(o.tools))
	toolMap := make(map[string]Tool, len(o.tools))
	for i, t := range o.tools {
		toolInfos[i] = t.toToolInfo()
		toolMap[t.Name] = t
	}
	bound, err := c.model.WithTools(toolInfos)
	if err != nil {
		return nil, fmt.Errorf("bind tools: %w", err)
	}
	go pumpWithTools(ctx, bound, in, toolMap, out)
	return out, nil
}

// buildSchemaMessages converts bite messages to eino's schema, prepending
// systemPrompt if it's non-empty and history lacks a system message.
func buildSchemaMessages(history []Message, systemPrompt string) ([]*schema.Message, error) {
	converted, err := toSchemaMessages(history)
	if err != nil {
		return nil, err
	}

	hasSystem := false
	for _, m := range history {
		if m.Role == RoleSystem {
			hasSystem = true
			break
		}
	}

	out := make([]*schema.Message, 0, len(converted)+1)
	if systemPrompt != "" && !hasSystem {
		out = append(out, &schema.Message{Role: schema.System, Content: systemPrompt})
	}
	return append(out, converted...), nil
}

func pump(ctx context.Context, reader *schema.StreamReader[*schema.Message], out chan<- StreamEvent) {
	defer close(out)

	final, ok := drainReader(ctx, reader, out)
	if !ok {
		return
	}
	select {
	case out <- StreamEvent{Done: true, Final: final.Content}:
	case <-ctx.Done():
	}
}
