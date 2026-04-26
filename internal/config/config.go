package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// defaultSystemPrompt is the nutritionist persona injected on every chat turn
// unless the user overrides BITE_SYSTEM_PROMPT.
//
// Specific tool descriptions are auto-appended from internal/tools at chat
// startup — keep this prompt focused on persona, posture, and house style.
const defaultSystemPrompt = `You are bite, a friendly terminal AI nutritionist.
Help users track meals, understand calories and macros, and make healthy choices.
When the user reports eating something — even casually — call the appropriate logging tool to save it automatically.
When the user asks about intake, progress, goals, streaks, or wants to change a target, prefer calling tools over guessing.
Keep responses concise and scannable; this is a terminal, not a web app.`

// Config is bite's runtime configuration.
//
// All settings come from environment variables. To add a setting, add a
// struct field with the right `env:"…"` tag — no other wiring needed.
type Config struct {
	DSN          string `env:"BITE_DB"`
	APIKey       string `env:"ANTHROPIC_API_KEY"`
	OpenAIAPIKey string `env:"OPENAI_API_KEY"`
	Model        string `env:"BITE_MODEL"          envDefault:"claude-sonnet-4-6"`
	MaxTokens    int    `env:"BITE_MAX_TOKENS"     envDefault:"4096"`
	SystemPrompt string `env:"BITE_SYSTEM_PROMPT"`
}

func Load() (Config, error) {
	_ = godotenv.Load()

	var c Config
	if err := env.Parse(&c); err != nil {
		return c, fmt.Errorf("parse env: %w", err)
	}

	if c.MaxTokens <= 0 {
		return c, fmt.Errorf("BITE_MAX_TOKENS must be positive, got %d", c.MaxTokens)
	}

	if c.SystemPrompt == "" {
		c.SystemPrompt = defaultSystemPrompt
	}

	if c.DSN == "" {
		path, err := xdg.DataFile(filepath.Join("bite", "bite.db"))
		if err != nil {
			return c, fmt.Errorf("resolve data dir: %w", err)
		}
		c.DSN = path
	} else if err := os.MkdirAll(filepath.Dir(c.DSN), 0o755); err != nil {
		return c, fmt.Errorf("create data dir: %w", err)
	}

	return c, nil
}

func (c Config) RequireAPIKey() error {
	if c.APIKey == "" {
		return errors.New("missing ANTHROPIC_API_KEY — export your key, or add it to a .env file:\n\n  export ANTHROPIC_API_KEY=sk-ant-…")
	}
	return nil
}
