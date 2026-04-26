package cli

import (
	"context"
	"fmt"
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
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

// runChat is also called by the root command (bite with no args).
func runChat(ctx context.Context, resumeID int64) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	store, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	client, err := openAIClient(ctx, cfg)
	if err != nil {
		return err
	}

	convID, history, err := prepareConversation(ctx, store, cfg.Model, resumeID)
	if err != nil {
		return err
	}

	persist := &chatPersister{
		store:          store,
		conversationID: convID,
		// Existing history means a previous user turn already set the title.
		titled: len(history) > 0,
	}

	deps := tools.Deps{Store: store, AI: client, OpenAIAPIKey: cfg.OpenAIAPIKey}
	streamOpts := []ai.StreamOption{ai.WithTools(tools.AITools(deps))}
	slash := func(c context.Context, line string) (string, error) {
		out := tools.Dispatch(c, deps, line)
		if out.ParseError != nil {
			return "", out.ParseError
		}
		if out.RunError != nil {
			return "", out.RunError
		}
		// AI rendering produces text + optional Markdown table — same shape
		// the model would see, which is what we want in transcript history.
		return tools.RenderResultForChat(out.Result), nil
	}
	prog := tui.New(ctx, client, persist, history, streamOpts, tui.WithSlashHandler(slash))
	_, err = prog.Run()
	return err
}

// prepareConversation either resumes an existing conversation (loading its
// history) or creates a fresh one. Centralised here so cli/chat.go and any
// future entry points (e.g. a REST mode) can share it.
func prepareConversation(ctx context.Context, store db.Storer, model string, resumeID int64) (int64, []ai.Message, error) {
	if resumeID > 0 {
		conv, err := store.GetConversation(ctx, resumeID)
		if err != nil {
			return 0, nil, fmt.Errorf("resume %d: %w", resumeID, err)
		}
		msgs, err := store.ListMessages(ctx, conv.ID)
		if err != nil {
			return 0, nil, fmt.Errorf("load history: %w", err)
		}
		history := make([]ai.Message, 0, len(msgs))
		for _, m := range msgs {
			history = append(history, ai.Message{Role: ai.Role(m.Role), Content: m.Content})
		}
		return conv.ID, history, nil
	}

	conv, err := store.NewConversation(ctx, model, "")
	if err != nil {
		return 0, nil, fmt.Errorf("new conversation: %w", err)
	}
	return conv.ID, nil, nil
}

// chatPersister is the smallest possible adapter from tui.Persister to db.Storer.
// It exists so the TUI never grows database knowledge — append a method here
// when the chat needs a new persistence behaviour (titles, summaries, etc.).
type chatPersister struct {
	store          db.Storer
	conversationID int64
	titled         bool // true once a title has been derived from a user message
}

func (p *chatPersister) AppendUser(ctx context.Context, content string) error {
	if _, err := p.store.AppendMessage(ctx, p.conversationID, "user", content); err != nil {
		return err
	}
	if !p.titled {
		p.titled = true
		// Title is cosmetic — swallow the error so a rename failure doesn't
		// abort the user's message.
		_ = p.store.RenameConversation(ctx, p.conversationID, deriveTitle(content))
	}
	return nil
}

func (p *chatPersister) AppendAssistant(ctx context.Context, content string) error {
	_, err := p.store.AppendMessage(ctx, p.conversationID, "assistant", content)
	return err
}

// deriveTitle picks a short, single-line title from the first user message.
// runewidth.Truncate handles unicode width correctly so we don't slice mid-rune.
func deriveTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return runewidth.Truncate(s, 60, "…")
}
