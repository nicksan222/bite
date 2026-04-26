package tools

import (
	"context"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
)

func init() {
	RegisterCheck(Check{
		Name:     "ai: ping model",
		Severity: SeverityHard,
		Desc:     "the model responds to a tiny test call",
		Gate:     "ping",
		Run: func(ctx context.Context) (string, error) {
			cfg, err := config.Load()
			if err != nil {
				return "", err
			}
			if err := cfg.RequireAPIKey(); err != nil {
				return "", err
			}
			client, err := ai.NewClient(ctx, ai.ClientConfig{
				APIKey: cfg.APIKey, Model: cfg.Model, MaxTokens: 16,
			})
			if err != nil {
				return "", err
			}
			ch, err := client.Stream(ctx, []ai.Message{
				{Role: ai.RoleUser, Content: "say 'pong' and nothing else"},
			})
			if err != nil {
				return "", err
			}
			for ev := range ch {
				if ev.Err != nil {
					return "", ev.Err
				}
				if ev.Done {
					break
				}
			}
			return "model responded", nil
		},
	})
}
