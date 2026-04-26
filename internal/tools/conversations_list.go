package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:    "conversations_list",
		Summary: "List saved conversations (most recent first).",
		Description: `Return up to N saved conversations (default 100), sorted by most recent
update.`,
		Prompt: `Use conversations_list when the user asks what they have been talking about,
wants to find a past chat by title, or wants the most recent conversation IDs.`,
		Params: []Param{
			{Name: "limit", Type: ParamInt,
				Desc: "Max conversations to list (default 100)."},
		},
		Run: runConversationsList,
	})
}

func runConversationsList(ctx context.Context, deps Deps, args Args) (Result, error) {
	limit := args.Int("limit")
	if limit <= 0 {
		limit = 100
	}
	rows, err := deps.Store.ListConversations(ctx, limit)
	if err != nil {
		return Result{}, fmt.Errorf("list conversations: %w", err)
	}
	if len(rows) == 0 {
		return Result{Text: "no conversations yet"}, nil
	}
	tbl := &Table{Headers: []string{"ID", "UPDATED", "MODEL", "TITLE"}}
	for _, c := range rows {
		title := c.Title
		if title == "" {
			title = "(untitled)"
		}
		tbl.Rows = append(tbl.Rows, []string{
			fmt.Sprintf("%d", c.ID),
			c.UpdatedAt.Format("2006-01-02 15:04"),
			c.Model,
			title,
		})
	}
	return Result{Table: tbl}, nil
}
