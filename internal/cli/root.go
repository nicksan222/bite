package cli

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

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
	tools.RegisterCobra(rootCmd, tools.CobraDepsProvider)
	return fang.Execute(
		ctx,
		rootCmd,
		fang.WithVersion(buildVersion),
		fang.WithCommit(buildCommit),
		fang.WithNotifySignal(os.Interrupt),
	)
}
