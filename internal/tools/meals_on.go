package tools

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register(Tool{
		Name:    "meals_on",
		Summary: "Show meals logged on a specific date (YYYY-MM-DD).",
		Description: `Return all meals logged on the given calendar date with calorie and macro
totals. Date format is YYYY-MM-DD in the user's local timezone.`,
		Prompt: `Call meals_on when the user asks about a specific past date ("what did I eat
on the 23rd?"). Use the user's local timezone.`,
		Params: []Param{
			{Name: "date", Type: ParamString, Required: true, Positional: true,
				Desc: "Calendar date in YYYY-MM-DD."},
		},
		Run: runMealsOn,
	})
}

func runMealsOn(ctx context.Context, deps Deps, args Args) (Result, error) {
	loc := deps.LocOrDefault()
	day, err := time.ParseInLocation("2006-01-02", args.String("date"), loc)
	if err != nil {
		return Result{}, fmt.Errorf("invalid date — expected YYYY-MM-DD: %w", err)
	}
	meals, err := deps.Store.ListMealsForDay(ctx, day, loc)
	if err != nil {
		return Result{}, fmt.Errorf("list meals: %w", err)
	}
	if len(meals) == 0 {
		return Result{Text: fmt.Sprintf("no meals logged on %s", day.Format("2006-01-02"))}, nil
	}
	var sb strings.Builder
	var tk, tp, tc, tf float64
	fmt.Fprintf(&sb, "%s:\n", day.Format("Monday 2006-01-02"))
	for _, m := range meals {
		fmt.Fprintf(&sb, "  - %s: %.0f kcal, %.1fg P, %.1fg C, %.1fg F\n",
			m.Title, m.Kcal, m.ProteinG, m.CarbsG, m.FatG)
		tk += m.Kcal
		tp += m.ProteinG
		tc += m.CarbsG
		tf += m.FatG
	}
	fmt.Fprintf(&sb, "Total: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat", tk, tp, tc, tf)
	return Result{Text: sb.String()}, nil
}
