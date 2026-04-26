package tools

import "context"

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

// runChat reuses the cobra-built Deps so the store opened by CobraDepsProvider
// flows straight into the TUI — no second open, no duplicated migration.
func runChat(ctx context.Context, deps Deps, args Args) (Result, error) {
	if err := RunChatWithDeps(ctx, deps, args.Int("resume")); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}
