package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:        "streak",
		Summary:     "Show the current daily logging streak.",
		Description: `Return the number of consecutive days the user has logged at least one meal.`,
		Prompt: `Call streak when the user asks about their streak, consistency, or tracking habits;
also useful when celebrating milestones or encouraging a return to consistency.`,
		Run: runStreak,
	})
}

func runStreak(ctx context.Context, deps Deps, _ Args) (Result, error) {
	n, err := deps.Store.Streak(ctx, deps.NowOrDefault(), deps.LocOrDefault())
	if err != nil {
		return Result{}, fmt.Errorf("get streak: %w", err)
	}
	switch n {
	case 0:
		return Result{Text: "No active streak — log a meal today to start one!"}, nil
	case 1:
		return Result{Text: "Current streak: 1 day. Keep it up!"}, nil
	default:
		return Result{Text: fmt.Sprintf("Current streak: %d days. Great consistency!", n)}, nil
	}
}
