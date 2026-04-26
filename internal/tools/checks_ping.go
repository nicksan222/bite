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
			cfg.MaxTokens = 16
			client, err := BuildAIClient(ctx, cfg)
			if err != nil {
				return "", err
			}
			// Skip the persona for the ping — the user message alone is
			// enough to elicit "pong".
			ch, err := client.Stream(ctx,
				[]ai.Message{{Role: ai.RoleUser, Content: "say 'pong' and nothing else"}},
				ai.WithSystemPrompt(""))
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
			spec, _ := ai.Resolve(ai.Provider(cfg.Provider))
			return "model responded via " + string(spec.Name), nil
		},
	})
}
