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

By default a fresh conversation is created. Pass --resume <id> to continue a
saved conversation. Run "bite --help" to see every command for finding ids.

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
				Desc: "Resume an existing conversation by id."},
		},
		Run: runChat,
	})
}

// runChat is the single source of truth for launching the chat TUI. Both
// `bite` (via SetDefault) and `bite chat` (via the named subcommand) reach
// here through the same cobra closure, so there's exactly one place that
// wires the registry into stream options + slash dispatch and runs the
// bubbletea program.
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
	_, err = prog.Run()
	return Result{}, err
}
