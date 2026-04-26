package tools

import (
	"context"
	"fmt"
	"strings"

	runewidth "github.com/mattn/go-runewidth"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
)

// RunChatTUI is the no-cobra entry point used by `bite` (no subcommand). It
// reuses CobraDepsProvider for deps so there's exactly ONE deps-builder
// shared with the cobra path, then dispatches through the registered chat
// Tool — the only function that actually knows how to launch the TUI.
func RunChatTUI(ctx context.Context, resumeID int64) error {
	deps, cleanup, err := CobraDepsProvider(ctx)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	chat := MustGet("chat")
	_, err = chat.Run(ctx, deps, NewArgsForTool(chat, map[string]any{"resume": resumeID}))
	return err
}

// PrepareSession resumes an existing conversation when resumeID > 0, otherwise
// creates a fresh one. Returns the conversation ID and any prior history.
//
// This is shared chat-orchestration glue, not a Tool — it doesn't fit the
// (ctx, Deps, Args) → Result shape because it needs the model name and
// returns multi-valued state. Lives here so any future entry point (REST,
// daemon, second TUI) can reuse it without touching cli/.
func PrepareSession(ctx context.Context, store db.Storer, model string, resumeID int64) (int64, []ai.Message, error) {
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

// NewChatPersister returns a struct that satisfies the TUI's Persister
// contract (AppendUser/AppendAssistant). titled tracks whether the
// conversation already has a title, so the first user turn auto-derives one.
func NewChatPersister(store db.Storer, conversationID int64, titled bool) *ChatPersister {
	return &ChatPersister{store: store, conversationID: conversationID, titled: titled}
}

// ChatPersister adapts db.Storer to the small surface the TUI needs.
type ChatPersister struct {
	store          db.Storer
	conversationID int64
	titled         bool
}

func (p *ChatPersister) AppendUser(ctx context.Context, content string) error {
	if _, err := p.store.AppendMessage(ctx, p.conversationID, "user", content); err != nil {
		return err
	}
	if !p.titled {
		p.titled = true
		// Title is cosmetic — swallow the error so a rename failure doesn't
		// abort the user's message.
		_ = p.store.RenameConversation(ctx, p.conversationID, DeriveTitle(content))
	}
	return nil
}

func (p *ChatPersister) AppendAssistant(ctx context.Context, content string) error {
	_, err := p.store.AppendMessage(ctx, p.conversationID, "assistant", content)
	return err
}

// DeriveTitle picks a short, single-line title from the first user message.
// runewidth.Truncate handles unicode width correctly so we don't slice mid-rune.
func DeriveTitle(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return runewidth.Truncate(s, 60, "…")
}
