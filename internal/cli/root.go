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
)

// SetBuildInfo lets cmd/bite/main.go inject ldflag-baked identity. Empty
// strings are ignored so a partial inject doesn't blank out a previously
// set value.
func SetBuildInfo(version, commit string) {
	if version != "" {
		buildVersion = version
	}
	if commit != "" {
		buildCommit = commit
	}
}

var rootCmd = &cobra.Command{
	Use:   "bite",
	Short: "bite — terminal AI nutritionist",
	Long: `bite is your terminal AI nutritionist.

Chat about food, log meals, estimate calories and macros from images, audio,
or video. Conversations live locally in SQLite.

Set an AI provider's API key before running:
  ANTHROPIC_API_KEY (default), OPENAI_API_KEY, GEMINI_API_KEY,
  or BITE_PROVIDER=ollama for a local Ollama daemon.
See bite doctor for environment health checks.`,
	// Example block + RunE are filled by tools.RegisterCobra and
	// tools.SetDefault respectively — see Execute below. This keeps the
	// rootCmd declarative; adding/renaming the chat tool needs no edits here.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute(ctx context.Context) error {
	tools.RegisterCobra(rootCmd, tools.CobraDepsProvider)
	tools.SetDefault(rootCmd, "chat")
	return fang.Execute(
		ctx,
		rootCmd,
		fang.WithVersion(buildVersion),
		fang.WithCommit(buildCommit),
		fang.WithNotifySignal(os.Interrupt),
	)
}
