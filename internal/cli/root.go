package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/db"
	"github.com/nicksan222/bite/internal/tools"
)

var (
	buildVersion = "dev"
	buildCommit  = "none"
	buildDate    = "unknown"
)

// SetBuildInfo lets cmd/bite/main.go inject ldflag-baked identity.
func SetBuildInfo(version, commit, date string) {
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
	if date != "" {
		buildDate = date
	}
}

var rootCmd = &cobra.Command{
	Use:   "bite",
	Short: "bite — terminal AI nutritionist",
	Long: `bite is your terminal AI nutritionist.

Chat about food, log meals, estimate calories and macros from images, audio,
or video. Conversations live locally in SQLite. See CLAUDE.md for where each
kind of change goes.

Set ANTHROPIC_API_KEY (or put it in .env) before running.`,
	Example: `  bite                                       # interactive chat (default)
  bite log_meal "200g pasta with pesto"       # log a meal from text
  bite log_meal_from_media "lunch" --file plate.jpg
  bite meals_today                            # today's intake summary
  bite ask "kcal in 200g salmon?"             # one-shot question
  bite doctor --ping                          # verify environment + model reachability`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runChat(cmd.Context(), 0)
	},
}

func Execute(ctx context.Context) error {
	tools.RegisterCobra(rootCmd, depsForCobra)
	return fang.Execute(
		ctx,
		rootCmd,
		fang.WithVersion(buildVersion),
		fang.WithCommit(buildCommit),
		fang.WithNotifySignal(os.Interrupt),
	)
}

// depsForCobra is invoked at the start of each tool subcommand. Opening the
// store and AI client lazily means `bite --help` works without an API key
// or a writable data dir.
func depsForCobra(ctx context.Context) (tools.Deps, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return tools.Deps{}, nil, err
	}
	store, err := openStore(ctx, cfg)
	if err != nil {
		return tools.Deps{}, nil, err
	}
	cleanup := func() { _ = store.Close() }

	deps := tools.Deps{
		Store:        store,
		OpenAIAPIKey: cfg.OpenAIAPIKey,
	}
	// AI is only opened if needed by the tool — log_meal_from_media is the
	// only cobra path that uses it, and it constructs its own client inside
	// the tool body via tools.LoadAI(deps).
	deps.AI = lazyAI{cfg: cfg}
	return deps, cleanup, nil
}

// lazyAI defers ai.Client construction until Stream is called. This keeps
// `bite --help` and tools that don't need the model (like meals_today) from
// failing when ANTHROPIC_API_KEY is unset.
type lazyAI struct {
	cfg config.Config
}

func (l lazyAI) Stream(ctx context.Context, history []ai.Message, opts ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	c, err := openAIClient(ctx, l.cfg)
	if err != nil {
		return nil, err
	}
	return c.Stream(ctx, history, opts...)
}

// ─── shared helpers used by subcommand files ────────────────────────────────

func loadConfig() (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}

func openStore(ctx context.Context, cfg config.Config) (*db.Store, error) {
	return db.Open(ctx, cfg.DSN)
}

func openAIClient(ctx context.Context, cfg config.Config) (*ai.Client, error) {
	if err := cfg.RequireAPIKey(); err != nil {
		return nil, err
	}
	return ai.NewClient(ctx, ai.ClientConfig{
		APIKey:       cfg.APIKey,
		Model:        cfg.Model,
		MaxTokens:    cfg.MaxTokens,
		SystemPrompt: cfg.SystemPrompt + tools.RenderAppendix(),
	})
}
