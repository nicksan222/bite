package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	Register(Tool{
		Name:        "meals_week",
		Summary:     "Show this week's meals with daily and weekly totals.",
		Description: `Return all meals logged this week (Mon–today) with daily and weekly totals.`,
		Prompt: `Call meals_week when the user asks about their weekly intake, weekly progress,
or how they are tracking across the full week.`,
		Examples: []Example{
			{Cmd: "bite meals_week", Desc: "this week's meals grouped by day"},
		},
		Run: runMealsWeek,
	})
}

func runMealsWeek(ctx context.Context, deps Deps, _ Args) (Result, error) {
	loc := deps.LocOrDefault()
	from := weekStart(deps.NowOrDefault(), loc)
	to := from.AddDate(0, 0, 7)
	meals, err := deps.Store.ListMealsBetween(ctx, from, to)
	if err != nil {
		return Result{}, fmt.Errorf("list meals: %w", err)
	}
	if len(meals) == 0 {
		return Result{Text: "No meals logged this week."}, nil
	}

	order := []string{}
	groups := map[string][]db.Meal{}
	for _, m := range meals {
		label := m.EatenAt.In(loc).Format("Monday 2006-01-02")
		if _, seen := groups[label]; !seen {
			order = append(order, label)
		}
		groups[label] = append(groups[label], m)
	}

	var sb strings.Builder
	var week totals
	for _, label := range order {
		fmt.Fprintf(&sb, "%s:\n", label)
		var day totals
		for _, m := range groups[label] {
			writeMealLine(&sb, "  ", m)
			day.add(m)
		}
		writeTotalsLine(&sb, "  Day total", day)
		sb.WriteByte('\n')
		week.Kcal += day.Kcal
		week.ProteinG += day.ProteinG
		week.CarbsG += day.CarbsG
		week.FatG += day.FatG
	}
	writeTotalsLine(&sb, "Week total", week)
	return Result{Text: sb.String()}, nil
}

// weekStart returns the Monday of the week containing t in loc.
func weekStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	y, m, d := t.Date()
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	return time.Date(y, m, d-wd+1, 0, 0, 0, 0, loc)
}
