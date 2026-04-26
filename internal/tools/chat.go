package tools

import (
	"context"

	"github.com/nicksan222/bite/internal/tui"
)

func init() {
	Register(Tool{
		Name:    "chat",
		Summary: "Open the interactive chat TUI.",
		Description: `Open the interactive chat TUI.

By default a fresh conversation is created. Use --resume <id> to continue an
existing one (see "bite conversations_list" for ids).

Inside chat, type /<name> to invoke any registered tool directly (deterministic,
no model call). Type /help to list every available slash command.`,
		// chat is a cobra-only command: as an AI tool it'd be recursive (the
		// model is *inside* the chat), and as /chat it'd spawn a TUI inside
		// the TUI. Skip both surfaces.
		SkipAI:    true,
		SkipSlash: true,
		Examples: []Example{
			{Cmd: "bite chat", Desc: "start a fresh chat"},
			{Cmd: "bite chat --resume 5", Desc: "resume conversation #5"},
		},
		Params: []Param{
			{Name: "resume", Type: ParamInt,
				Desc: "Resume an existing conversation by id (see bite conversations_list)."},
		},
		Run: runChat,
	})
}

// runChat is the single source of truth for launching the chat TUI. Both
// `bite` (via RunChatTUI) and `bite chat` (via the cobra adapter) end up
// here, so there's exactly one place that wires the registry into stream
// options + slash dispatch and runs the bubbletea program.
//
// Validates the AI client up front via Deps.RequireAI so a missing
// ANTHROPIC_API_KEY surfaces before the TUI opens.
func runChat(ctx context.Context, deps Deps, args Args) (Result, error) {
	if err := deps.RequireAI(); err != nil {
		return Result{}, err
	}
	convID, history, err := PrepareSession(ctx, deps.Store, deps.Model, args.Int("resume"))
	if err != nil {
		return Result{}, err
	}
	persist := NewChatPersister(deps.Store, convID, len(history) > 0)
	prog := tui.New(ctx, deps.AI, persist, history,
		ChatStreamOptions(deps),
		tui.WithSlashHandler(NewSlashHandler(deps)))
	if _, err := prog.Run(); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}
