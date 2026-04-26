package tools

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(Tool{
		Name:        "conversation_show",
		Summary:     "Print all messages in a conversation.",
		Description: `Render every message of one saved conversation in chronological order.`,
		Prompt: `Use conversation_show when the user asks to revisit a past chat, recall what
was said in conversation #N, or quote earlier guidance.`,
		Params: []Param{
			{Name: "conversation_id", Type: ParamInt, Required: true, Positional: true,
				Desc: "ID of the conversation to display."},
		},
		Run: runConversationShow,
	})
}

func runConversationShow(ctx context.Context, deps Deps, args Args) (Result, error) {
	id := args.Int("conversation_id")
	conv, err := deps.Store.GetConversation(ctx, id)
	if err != nil {
		return Result{}, fmt.Errorf("conversation %d: %w", id, err)
	}
	msgs, err := deps.Store.ListMessages(ctx, conv.ID)
	if err != nil {
		return Result{}, fmt.Errorf("load messages: %w", err)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# conversation %d (model %s)\n\n", conv.ID, conv.Model)
	for _, m := range msgs {
		fmt.Fprintf(&sb, "## %s\n\n%s\n\n", m.Role, m.Content)
	}
	return Result{Text: sb.String()}, nil
}
