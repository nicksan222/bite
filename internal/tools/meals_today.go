package tools

import (
	"context"
	"fmt"
	"strings"
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
	var totalKcal, totalP, totalC, totalF float64
	var sb strings.Builder
	for _, m := range meals {
		fmt.Fprintf(&sb, "- %s: %.0f kcal, %.1fg P, %.1fg C, %.1fg F\n",
			m.Title, m.Kcal, m.ProteinG, m.CarbsG, m.FatG)
		totalKcal += m.Kcal
		totalP += m.ProteinG
		totalC += m.CarbsG
		totalF += m.FatG
	}
	fmt.Fprintf(&sb, "Total: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
		totalKcal, totalP, totalC, totalF)
	return Result{Text: sb.String()}, nil
}
