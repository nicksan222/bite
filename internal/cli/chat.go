package cli

import (
	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/tools"
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
			return tools.RunChatTUI(cmd.Context(), resume)
		},
	}
	cmd.Flags().Int64P("resume", "r", 0, "resume an existing conversation by id")
	rootCmd.AddCommand(cmd)
}
