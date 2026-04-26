package tools

import (
	"context"
	"fmt"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/db"
)

// LoadConfig wraps config.Load with a uniform error prefix. Centralised so
// every entry point (cobra subcommands, chat launcher, future REST mode)
// reports configuration failures the same way.
func LoadConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

// OpenStore opens (and migrates) the SQLite database at cfg.DSN.
func OpenStore(ctx context.Context, cfg config.Config) (*db.Store, error) {
	return db.Open(ctx, cfg.DSN)
}

// OpenAIClient builds an *ai.Client from cfg with the assembled system
// prompt (default persona or BITE_SYSTEM_PROMPT override, plus the
// auto-generated tool appendix). Fails fast if ANTHROPIC_API_KEY is unset.
func OpenAIClient(ctx context.Context, cfg config.Config) (*ai.Client, error) {
	if err := cfg.RequireAPIKey(); err != nil {
		return nil, err
	}
	return ai.NewClient(ctx, ai.ClientConfig{
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		MaxTokens:    cfg.MaxTokens,
		SystemPrompt: BuildSystemPrompt(cfg.SystemPrompt),
	})
}

// CobraDepsProvider returns a DepsProvider suitable for tools.RegisterCobra.
// It opens the store eagerly (so failures surface before Run starts) and
// defers AI-client construction until a tool actually calls Stream — that
// way `bite --help` works with no API key and tools that don't need the
// model (meals_today, get_goals, …) don't trigger network setup.
func CobraDepsProvider(ctx context.Context) (Deps, func(), error) {
	cfg, err := LoadConfig()
	if err != nil {
		return Deps{}, nil, err
	}
	store, err := OpenStore(ctx, cfg)
	if err != nil {
		return Deps{}, nil, err
	}
	cleanup := func() { _ = store.Close() }
	return Deps{
		Store:        store,
		AI:           lazyAI{cfg: cfg},
		Model:        cfg.Model,
		OpenAIAPIKey: cfg.OpenAIAPIKey,
	}, cleanup, nil
}

// lazyAI is an ai.Streamer that constructs the real client only on first
// Stream. Used by CobraDepsProvider so tools without AI needs don't pay
// the API-key check up front.
type lazyAI struct {
	cfg config.Config
}

func (l lazyAI) Stream(ctx context.Context, history []ai.Message, opts ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	c, err := OpenAIClient(ctx, l.cfg)
	if err != nil {
		return nil, err
	}
	return c.Stream(ctx, history, opts...)
}
