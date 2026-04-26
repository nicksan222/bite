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
			provider := cfg.Provider
			if provider == "" {
				provider = "(auto)"
			}
			model := cfg.Model
			if model == "" {
				model = "(default)"
			}
			return fmt.Sprintf("DSN=%s provider=%s model=%s", cfg.DSN, provider, model), nil
		},
	})
}
