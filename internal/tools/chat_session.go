package tools

import (
	"context"
	"fmt"
	"strings"

	runewidth "github.com/mattn/go-runewidth"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
	"github.com/nicksan222/bite/internal/tui"
)

// RunChatTUI is the full chat-launch flow: load config, open store, then run
// the bubbletea program. Used by cli/root.go's no-arg `bite` invocation. The
// AI client is built lazily — RunChatWithDeps' RequireAI gate still ensures
// a missing ANTHROPIC_API_KEY errors before the TUI opens, while keeping
// this path symmetric with the cobra path (`bite chat`).
func RunChatTUI(ctx context.Context, resumeID int64) error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	store, err := OpenStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	deps := Deps{
		Store:        store,
		AI:           lazyAI{cfg: cfg},
		Model:        cfg.Model,
		OpenAIAPIKey: cfg.OpenAIAPIKey,
	}
	return RunChatWithDeps(ctx, deps, resumeID)
}

// RunChatWithDeps is the inner chat-launcher: given a fully-populated Deps
// (Store, AI, Model required), prepare a session and run the TUI. Splitting
// this out lets the chat Tool reuse the cobra-adapter's already-built deps
// instead of re-opening the store.
//
// Validates the AI client up front via Deps.RequireAI so a missing
// ANTHROPIC_API_KEY surfaces before the TUI opens.
func RunChatWithDeps(ctx context.Context, deps Deps, resumeID int64) error {
	if err := deps.RequireAI(); err != nil {
		return err
	}
	convID, history, err := PrepareSession(ctx, deps.Store, deps.Model, resumeID)
	if err != nil {
		return err
	}
	persist := NewChatPersister(deps.Store, convID, len(history) > 0)
	prog := tui.New(ctx, deps.AI, persist, history,
		ChatStreamOptions(deps),
		tui.WithSlashHandler(NewSlashHandler(deps)))
	_, err = prog.Run()
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
