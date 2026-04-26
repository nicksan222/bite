package ai

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

func init() {
	RegisterProvider(ProviderSpec{
		Name:          ProviderOpenAI,
		DefaultModel:  "gpt-4o-mini",
		CredentialEnv: "OPENAI_API_KEY",
		Priority:      20,
		Build:         buildOpenAIModel,
	})
}

func buildOpenAIModel(ctx context.Context, cfg ProviderConfig) (model.ToolCallingChatModel, error) {
	c := &openai.ChatModelConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	}
	if cfg.MaxTokens > 0 {
		c.MaxTokens = &cfg.MaxTokens
	}
	cm, err := openai.NewChatModel(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("openai: new chat model: %w", err)
	}
	return cm, nil
}
