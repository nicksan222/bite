package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/tools"
	"github.com/nicksan222/bite/internal/tui"
)

func init() {
	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session",
		Long: `Open the interactive chat TUI.

By default a fresh conversation is created. Use --resume <id> to continue an
existing one (see "bite conversations_list" for ids).

Inside chat, type /<name> to invoke any registered tool directly (deterministic,
no model call). Type /help to list every available slash command.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resume, _ := cmd.Flags().GetInt64("resume")
			return runChat(cmd.Context(), resume)
		},
	}
	cmd.Flags().Int64P("resume", "r", 0, "resume an existing conversation by id")
	rootCmd.AddCommand(cmd)
}

// runChat is also called by the root command (bite with no args). All the
// orchestration helpers (LoadConfig, OpenStore, OpenAIClient, PrepareSession,
// NewChatPersister, Dispatch) live in internal/tools so this file stays a
// thin TUI launcher.
func runChat(ctx context.Context, resumeID int64) error {
	cfg, err := tools.LoadConfig()
	if err != nil {
		return err
	}
	store, err := tools.OpenStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	client, err := tools.OpenAIClient(ctx, cfg)
	if err != nil {
		return err
	}

	convID, history, err := tools.PrepareSession(ctx, store, cfg.Model, resumeID)
	if err != nil {
		return err
	}

	deps := tools.Deps{Store: store, AI: client, OpenAIAPIKey: cfg.OpenAIAPIKey}
	streamOpts := []ai.StreamOption{ai.WithTools(tools.AITools(deps))}
	persist := tools.NewChatPersister(store, convID, len(history) > 0)
	prog := tui.New(ctx, client, persist, history, streamOpts,
		tui.WithSlashHandler(tools.NewSlashHandler(deps)))
	_, err = prog.Run()
	return err
}
