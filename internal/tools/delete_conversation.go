package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:        "delete_conversation",
		Summary:     "Delete a saved conversation and all its messages.",
		Description: `Remove a saved conversation and every message it contains. Irreversible.`,
		Prompt: `Use delete_conversation when the user explicitly asks to delete a past chat.
Always confirm with the user before calling — this cannot be undone.`,
		Examples: []Example{
			{Cmd: "bite delete_conversation 12", Desc: "delete a saved chat by id"},
		},
		Params: []Param{
			{Name: "conversation_id", Type: ParamInt, Required: true, Positional: true,
				Desc: "ID of the conversation to delete."},
		},
		Run: runDeleteConversation,
	})
}

func runDeleteConversation(ctx context.Context, deps Deps, args Args) (Result, error) {
	id := args.Int("conversation_id")
	if err := deps.Store.DeleteConversation(ctx, id); err != nil {
		return Result{}, fmt.Errorf("delete conversation: %w", err)
	}
	return Result{Text: fmt.Sprintf("deleted conversation %d", id)}, nil
}
