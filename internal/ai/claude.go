package ai

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/claude"
	"github.com/cloudwego/eino/components/model"
)

func init() {
	RegisterProvider(ProviderSpec{
		Name:          ProviderAnthropic,
		DefaultModel:  "claude-sonnet-4-6",
		CredentialEnv: "ANTHROPIC_API_KEY",
		Priority:      10,
		Build:         buildClaudeModel,
	})
}

func buildClaudeModel(ctx context.Context, cfg ProviderConfig) (model.ToolCallingChatModel, error) {
	cm, err := claude.NewChatModel(ctx, &claude.Config{
		APIKey:    cfg.APIKey,
		Model:     cfg.Model,
		MaxTokens: cfg.MaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("claude: new chat model: %w", err)
	}
	return cm, nil
}
