package db

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all up migrations embedded in the binary. Goose's default
// logger prints "OK <migration>.sql" lines on every Migrate call, which is
// noisy at the CLI (every `bite ...` invocation triggers Migrate). Silence
// it — wrapped errors carry enough context to debug failures.
func Migrate(d *sql.DB) error {
	goose.SetLogger(goose.NopLogger())
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(d, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
