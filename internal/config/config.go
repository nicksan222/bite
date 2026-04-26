package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adrg/xdg"
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

// Config is bite's runtime configuration. Every setting is an env var;
// add a field with the right `env:"…"` tag to add a knob.
//
// Per-provider AI credentials are NOT parsed here — each provider in
// internal/ai reads its own env var so adding a backend is a one-file
// change. OpenAIAPIKey is the exception: media/Whisper transcription
// uses it regardless of the chat provider.
type Config struct {
	DSN          string `env:"BITE_DB"`
	Provider     string `env:"BITE_PROVIDER"`
	OpenAIAPIKey string `env:"OPENAI_API_KEY"`
	Model        string `env:"BITE_MODEL"`
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
