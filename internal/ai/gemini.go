package ai

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/gemini"
	"github.com/cloudwego/eino/components/model"
	"google.golang.org/genai"
)

func init() {
	RegisterProvider(ProviderSpec{
		Name:          ProviderGemini,
		DefaultModel:  "gemini-2.5-flash",
		CredentialEnv: "GEMINI_API_KEY",
		Priority:      30,
		Build:         buildGeminiModel,
	})
}

func buildGeminiModel(ctx context.Context, cfg ProviderConfig) (model.ToolCallingChatModel, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: cfg.APIKey})
	if err != nil {
		return nil, fmt.Errorf("gemini: new client: %w", err)
	}
	c := &gemini.Config{Client: client, Model: cfg.Model}
	if cfg.MaxTokens > 0 {
		c.MaxTokens = &cfg.MaxTokens
	}
	cm, err := gemini.NewChatModel(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("gemini: new chat model: %w", err)
	}
	return cm, nil
}
