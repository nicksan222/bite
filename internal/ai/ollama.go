package ai

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"
)

// DefaultOllamaBaseURL is the address Ollama listens on by default.
const DefaultOllamaBaseURL = "http://localhost:11434"

func init() {
	RegisterProvider(ProviderSpec{
		Name:           ProviderOllama,
		DefaultModel:   "qwen3:4b",
		BaseURLEnv:     "OLLAMA_BASE_URL",
		DefaultBaseURL: DefaultOllamaBaseURL,
		Priority:       40,
		Build:          buildOllamaModel,
	})
}

func buildOllamaModel(ctx context.Context, cfg ProviderConfig) (model.ToolCallingChatModel, error) {
	cm, err := ollama.NewChatModel(ctx, &ollama.ChatModelConfig{
		BaseURL: cfg.BaseURL,
		Model:   cfg.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("ollama: new chat model: %w", err)
	}
	return cm, nil
}
