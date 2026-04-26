package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:    "meals_today",
		Summary: "Show today's logged meals with calorie and macro totals.",
		Description: `Return all meals logged today with their calorie and macro totals.
No parameters needed.`,
		Prompt: `Call meals_today when the user asks about today's intake, remaining budget,
or progress toward a daily goal.`,
		Run: runMealsToday,
	})
}

func runMealsToday(ctx context.Context, deps Deps, _ Args) (Result, error) {
	meals, err := deps.Store.ListMealsForDay(ctx, deps.NowOrDefault(), deps.LocOrDefault())
	if err != nil {
		return Result{}, fmt.Errorf("list meals: %w", err)
	}
	if len(meals) == 0 {
		return Result{Text: "No meals logged today."}, nil
	}
	return Result{Text: formatMealList(meals)}, nil
}
