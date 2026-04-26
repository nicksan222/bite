package tools

import (
	"context"
	"fmt"

	"github.com/nicksan222/bite/internal/config"
)

func init() {
	RegisterCheck(Check{
		Name:     "config: load",
		Severity: SeverityHard,
		Desc:     "configuration parses from environment + .env",
		Run: func(_ context.Context) (string, error) {
			cfg, err := config.Load()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("DSN=%s model=%s", cfg.DSN, cfg.Model), nil
		},
	})

	RegisterCheck(Check{
		Name:     "config: api key",
		Severity: SeverityHard,
		Desc:     "ANTHROPIC_API_KEY is set",
		Run: func(_ context.Context) (string, error) {
			cfg, err := config.Load()
			if err != nil {
				return "", err
			}
			if err := cfg.RequireAPIKey(); err != nil {
				return "", err
			}
			return fmt.Sprintf("ANTHROPIC_API_KEY set (len=%d)", len(cfg.APIKey)), nil
		},
	})
}
