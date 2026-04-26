package ai

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
)

// newClaudeModel constructs the underlying Claude chat model used by Client.
//
// bite is Claude-only by design. If you ever need a second provider, add a
// sibling file (e.g. internal/ai/openai.go) with the same shape and select
// between them in NewClient.
func newClaudeModel(ctx context.Context, apiKey, modelID string, maxTokens int) (model.ToolCallingChatModel, error) {
	if apiKey == "" {
		return nil, errors.New("claude: missing API key")
	}
	if modelID == "" {
		return nil, errors.New("claude: missing model id")
	}
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		APIKey:    apiKey,
		Model:     modelID,
		MaxTokens: maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("claude: new chat model: %w", err)
	}
	return cm, nil
}
