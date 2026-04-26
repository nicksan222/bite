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
	Example: `  bite                                  # interactive chat (default)
  bite analyze meal.jpg                  # estimate kcal + macros from a photo
  bite analyze plate.jpg note.m4a -m "lunch"  # mix media + text
  bite ask "kcal in 200g salmon?"        # one-shot question
  bite doctor --ping                     # verify environment + model reachability`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return runChat(cmd.Context(), 0)
	},
}

func Execute(ctx context.Context) error {
	return fang.Execute(
		ctx,
		rootCmd,
		fang.WithVersion(buildVersion),
		fang.WithCommit(buildCommit),
		fang.WithNotifySignal(os.Interrupt),
	)
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
		SystemPrompt: cfg.SystemPrompt,
	})
}
