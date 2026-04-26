package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:        "rename_conversation",
		Summary:     "Rename a saved conversation.",
		Description: `Update the title of a saved conversation by ID.`,
		Prompt: `Use rename_conversation when the user wants to retitle a past chat for easier
recall ("rename conversation 12 to 'protein experiment'").`,
		Examples: []Example{
			{Cmd: `bite rename_conversation 12 "protein experiment"`, Desc: "retitle a saved chat"},
		},
		Params: []Param{
			{Name: "conversation_id", Type: ParamInt, Required: true, Positional: true,
				Desc: "ID of the conversation."},
			{Name: "title", Type: ParamString, Required: true, Positional: true,
				Desc: "New title."},
		},
		Run: runRenameConversation,
	})
}

func runRenameConversation(ctx context.Context, deps Deps, args Args) (Result, error) {
	id := args.Int("conversation_id")
	title := args.String("title")
	if err := deps.Store.RenameConversation(ctx, id, title); err != nil {
		return Result{}, fmt.Errorf("rename conversation: %w", err)
	}
	return Result{Text: fmt.Sprintf("renamed conversation %d → %q", id, title)}, nil
}
