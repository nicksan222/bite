package tools

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/db"
)

// LoadConfig wraps config.Load with a uniform error prefix so every
// entry point reports configuration failures the same way.
func LoadConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func OpenStore(ctx context.Context, cfg config.Config) (*db.Store, error) {
	return db.Open(ctx, cfg.DSN)
}

// BuildAIClient resolves cfg.Provider, validates the resolved provider's
// credentials, and constructs a streaming Client.
func BuildAIClient(ctx context.Context, cfg config.Config) (*ai.Client, error) {
	spec, err := ai.Resolve(ai.Provider(cfg.Provider))
	if err != nil {
		return nil, err
	}
	pcfg := spec.LoadConfig(cfg.Model, cfg.MaxTokens)
	if err := spec.Validate(pcfg); err != nil {
		return nil, err
	}
	return ai.NewClient(ctx, ai.ClientConfig{
		Provider:     spec.Name,
		APIKey:       pcfg.APIKey,
		BaseURL:      pcfg.BaseURL,
		Model:        pcfg.Model,
		MaxTokens:    pcfg.MaxTokens,
		SystemPrompt: BuildSystemPrompt(cfg.SystemPrompt),
	})
}

// CobraDepsProvider opens the store eagerly and defers AI-client
// construction until first Stream — so `bite --help` and tools that
// don't need the model don't trigger network setup or the bootstrap
// prompt.
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
		AI:           &lazyAI{cfg: cfg, hooks: defaultBootstrapHooks(store)},
		Model:        resolvedModel(cfg),
		OpenAIAPIKey: cfg.OpenAIAPIKey,
	}, cleanup, nil
}

func resolvedModel(cfg config.Config) string {
	if cfg.Model != "" {
		return cfg.Model
	}
	spec, err := ai.Resolve(ai.Provider(cfg.Provider))
	if err != nil {
		return ""
	}
	return spec.DefaultModel
}

// lazyAI defers Client construction (and the once-per-process Ollama
// bootstrap consent flow) until first use.
type lazyAI struct {
	cfg   config.Config
	hooks bootstrapHooks

	once     sync.Once
	resolved config.Config
	err      error
}

func (l *lazyAI) ensure(ctx context.Context) (config.Config, error) {
	l.once.Do(func() {
		l.resolved, l.err = ensureProviderOrBootstrap(ctx, l.cfg, l.hooks)
	})
	return l.resolved, l.err
}

func (l *lazyAI) Stream(ctx context.Context, history []ai.Message, opts ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	cfg, err := l.ensure(ctx)
	if err != nil {
		return nil, err
	}
	c, err := BuildAIClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return c.Stream(ctx, history, opts...)
}

func (l *lazyAI) EnsureUsable() error {
	_, err := l.ensure(context.Background())
	return err
}

// ResolvedModel returns the model the AI layer settled on, including any
// provider switch the bootstrap performed. Used by chat persistence so
// the conversation row records what actually answered.
func (l *lazyAI) ResolvedModel() string {
	cfg, err := l.ensure(context.Background())
	if err != nil {
		return resolvedModel(l.cfg)
	}
	return resolvedModel(cfg)
}

// ensureProviderOrBootstrap decides what to do when AI is needed:
// explicit BITE_PROVIDER=ollama runs the (idempotent) bootstrap with no
// prompt; an explicit non-ollama provider or any auto-detected
// credential falls through to Validate; otherwise the user is asked
// once, and a yes is persisted so future runs skip straight to
// bootstrap.
func ensureProviderOrBootstrap(ctx context.Context, cfg config.Config, h bootstrapHooks) (config.Config, error) {
	if cfg.Provider == string(ai.ProviderOllama) {
		if err := bootstrapOllama(ctx, cfg, h); err != nil {
			return cfg, err
		}
		return cfg, nil
	}

	if cfg.Provider != "" || ai.AnyConfigured() == nil {
		return cfg, validateExisting(cfg)
	}

	if h.HasConsent(ctx) {
		if err := bootstrapOllama(ctx, cfg, h); err != nil {
			return cfg, err
		}
		cfg.Provider = string(ai.ProviderOllama)
		return cfg, nil
	}

	if !h.PromptConsent() {
		return cfg, ai.AnyConfigured()
	}
	if err := bootstrapOllama(ctx, cfg, h); err != nil {
		return cfg, err
	}
	if err := h.RecordConsent(ctx); err != nil {
		// Best-effort: bootstrap already worked; we'll just re-prompt
		// next run if the write fails.
		fmt.Fprintln(os.Stderr, "warning:", err)
	}
	cfg.Provider = string(ai.ProviderOllama)
	return cfg, nil
}

func bootstrapOllama(ctx context.Context, cfg config.Config, h bootstrapHooks) error {
	spec, ok := ai.Lookup(ai.ProviderOllama)
	if !ok {
		return fmt.Errorf("ollama provider not registered")
	}
	pcfg := spec.LoadConfig(cfg.Model, cfg.MaxTokens)
	return h.Bootstrap(ctx, pcfg.Model, pcfg.BaseURL)
}

func validateExisting(cfg config.Config) error {
	spec, err := ai.Resolve(ai.Provider(cfg.Provider))
	if err != nil {
		return err
	}
	return spec.Validate(spec.LoadConfig(cfg.Model, cfg.MaxTokens))
}

// bootstrapHooks isolates side-effects so tests can stub them.
type bootstrapHooks struct {
	HasConsent    func(ctx context.Context) bool
	RecordConsent func(ctx context.Context) error
	PromptConsent func() bool
	Bootstrap     func(ctx context.Context, model, baseURL string) error
}

func defaultBootstrapHooks(store db.Storer) bootstrapHooks {
	return bootstrapHooks{
		HasConsent:    func(ctx context.Context) bool { return HasOllamaConsent(ctx, store) },
		RecordConsent: func(ctx context.Context) error { return RecordOllamaConsent(ctx, store) },
		PromptConsent: func() bool {
			return Confirm(os.Stdin, os.Stderr,
				"No AI provider keys configured. Install and run Ollama locally?")
		},
		Bootstrap: func(ctx context.Context, model, baseURL string) error {
			return ai.DefaultBootstrap(model, baseURL).Run(ctx, os.Stderr)
		},
	}
}

// aiEnsurer is duck-typed so test deps don't need a no-op method.
type aiEnsurer interface{ EnsureUsable() error }
