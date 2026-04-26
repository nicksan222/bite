package tools

import (
	"context"
	"path/filepath"

	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/db"
)

func init() {
	RegisterCheck(Check{
		Name:     "db: open + migrate",
		Severity: SeverityHard,
		Desc:     "SQLite is reachable and migrations apply",
		Run: func(ctx context.Context) (string, error) {
			cfg, err := config.Load()
			if err != nil {
				return "", err
			}
			store, err := db.Open(ctx, cfg.DSN)
			if err != nil {
				return "", err
			}
			_ = store.Close()
			return filepath.Base(cfg.DSN), nil
		},
	})
}
