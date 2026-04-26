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
		Examples: []Example{
			{Cmd: "bite meals_on 2026-04-20", Desc: "show meals logged on a specific date"},
		},
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
	fmt.Fprintf(&sb, "%s:\n", day.Format("Monday 2006-01-02"))
	sb.WriteString(formatMealList(meals))
	return Result{Text: sb.String()}, nil
}
