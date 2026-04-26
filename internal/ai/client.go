package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type Provider string

const (
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderGemini    Provider = "gemini"
	ProviderOllama    Provider = "ollama"
)

// ClientConfig is forwarded to the resolved ProviderSpec. Empty Provider
// triggers ai.Resolve.
type ClientConfig struct {
	Provider     Provider
	APIKey       string
	BaseURL      string
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
	spec, err := Resolve(cfg.Provider)
	if err != nil {
		return nil, err
	}
	pcfg := ProviderConfig{
		APIKey:    cfg.APIKey,
		BaseURL:   cfg.BaseURL,
		Model:     cfg.Model,
		MaxTokens: cfg.MaxTokens,
	}
	if pcfg.Model == "" {
		pcfg.Model = spec.DefaultModel
	}
	if pcfg.BaseURL == "" {
		pcfg.BaseURL = spec.DefaultBaseURL
	}
	if err := spec.Validate(pcfg); err != nil {
		return nil, err
	}
	m, err := spec.Build(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	return &Client{model: m, systemPrompt: cfg.SystemPrompt}, nil
}

// StreamEvent is one chunk delivered while a response is being generated.
// Exactly one of Delta, Err, Done, or ToolStep is set.
type StreamEvent struct {
	Delta string
	Err   error
	Done  bool
	Final string
	// ToolStep is set when the model calls a tool. UIs can render these as
	// "thinking" steps so the user sees what the model is doing between
	// the input and the final answer.
	ToolStep *ToolStep
}

// ToolStep describes a tool-call lifecycle event observed during streaming.
// One Started event is emitted as soon as the model requests a tool, then
// one Finished event after the tool returns. Both carry the same ID so UIs
// can update an in-place row instead of stacking duplicates.
type ToolStep struct {
	ID        string
	Name      string
	Arguments string // raw JSON args as supplied by the model
	Result    string // populated only when Finished is true
	Finished  bool
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
