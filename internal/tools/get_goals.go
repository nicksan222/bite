package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:        "get_goals",
		Summary:     "Show the user's daily nutrition targets.",
		Description: `Return the user's daily nutrition targets (kcal, protein, carbs, fat).`,
		Prompt: `Call get_goals before giving personalised advice that compares intake against
targets, or when the user asks how close they are to their goals or whether
they have room for another meal.`,
		Examples: []Example{
			{Cmd: "bite get_goals", Desc: "show current daily targets"},
		},
		Run: runGetGoals,
	})
}

func runGetGoals(ctx context.Context, deps Deps, _ Args) (Result, error) {
	g, err := deps.Store.GetGoals(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("get goals: %w", err)
	}
	return Result{Text: formatGoals("Daily targets:", g)}, nil
}
