package cli

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	convCmd := &cobra.Command{
		Use:     "conversations",
		Aliases: []string{"convos", "conv"},
		Short:   "Manage saved conversations",
	}

	convCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List saved conversations (most recent first)",
		RunE:  runConversationsList,
	})

	convCmd.AddCommand(&cobra.Command{
		Use:   "show <id>",
		Short: "Print all messages in a conversation",
		Args:  cobra.ExactArgs(1),
		RunE:  runConversationShow,
	})

	convCmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a conversation and its messages",
		Args:  cobra.ExactArgs(1),
		RunE:  runConversationDelete,
	})

	convCmd.AddCommand(&cobra.Command{
		Use:   "rename <id> <title>",
		Short: "Rename a conversation",
		Args:  cobra.ExactArgs(2),
		RunE:  runConversationRename,
	})

	rootCmd.AddCommand(convCmd)
}

func withStore(ctx context.Context, fn func(s db.Storer) error) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	store, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	return fn(store)
}

func parseID(s string) (int64, error) {
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q: %w", s, err)
	}
	return id, nil
}

func runConversationsList(cmd *cobra.Command, _ []string) error {
	return withStore(cmd.Context(), func(store db.Storer) error {
		return listConversations(cmd.Context(), cmd.OutOrStdout(), store)
	})
}

func listConversations(ctx context.Context, out io.Writer, store db.Storer) error {
	rows, err := store.ListConversations(ctx, 100)
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tUPDATED\tMODEL\tTITLE")
	for _, c := range rows {
		title := c.Title
		if title == "" {
			title = "(untitled)"
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", c.ID, c.UpdatedAt.Format("2006-01-02 15:04"), c.Model, title)
	}
	return tw.Flush()
}

func runConversationShow(cmd *cobra.Command, args []string) error {
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	return withStore(cmd.Context(), func(store db.Storer) error {
		return showConversation(cmd.Context(), cmd.OutOrStdout(), store, id)
	})
}

func showConversation(ctx context.Context, out io.Writer, store db.Storer, id int64) error {
	conv, err := store.GetConversation(ctx, id)
	if err != nil {
		return err
	}
	msgs, err := store.ListMessages(ctx, conv.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "# conversation %d (model %s)\n\n", conv.ID, conv.Model)
	for _, m := range msgs {
		fmt.Fprintf(out, "## %s\n\n%s\n\n", m.Role, m.Content)
	}
	return nil
}

func runConversationDelete(cmd *cobra.Command, args []string) error {
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	return withStore(cmd.Context(), func(store db.Storer) error {
		return deleteConversation(cmd.Context(), cmd.OutOrStdout(), store, id)
	})
}

func deleteConversation(ctx context.Context, out io.Writer, store db.Storer, id int64) error {
	if err := store.DeleteConversation(ctx, id); err != nil {
		return err
	}
	fmt.Fprintf(out, "deleted conversation %d\n", id)
	return nil
}

func runConversationRename(cmd *cobra.Command, args []string) error {
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	return withStore(cmd.Context(), func(store db.Storer) error {
		return renameConversation(cmd.Context(), cmd.OutOrStdout(), store, id, args[1])
	})
}

func renameConversation(ctx context.Context, out io.Writer, store db.Storer, id int64, title string) error {
	if err := store.RenameConversation(ctx, id, title); err != nil {
		return err
	}
	fmt.Fprintf(out, "renamed conversation %d → %q\n", id, title)
	return nil
}
