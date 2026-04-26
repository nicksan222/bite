package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:    "meals_recent",
		Summary: "Show the N most recent meals across all dates.",
		Description: `Return the N most recent meals across all dates with their calorie and
macro totals.`,
		Prompt: `Call meals_recent when the user asks for their last few meals regardless of
when they were eaten ("show my last 5 meals").`,
		Params: []Param{
			{Name: "limit", Type: ParamInt, Required: true, Positional: true,
				Desc: "How many meals to show."},
		},
		Run: runMealsRecent,
	})
}

func runMealsRecent(ctx context.Context, deps Deps, args Args) (Result, error) {
	limit := args.Int("limit")
	if limit <= 0 {
		return Result{}, fmt.Errorf("limit must be positive, got %d", limit)
	}
	meals, err := deps.Store.ListRecentMeals(ctx, limit)
	if err != nil {
		return Result{}, fmt.Errorf("list meals: %w", err)
	}
	if len(meals) == 0 {
		return Result{Text: "no meals logged yet"}, nil
	}

	loc := deps.LocOrDefault()
	tbl := &Table{
		Headers: []string{"ID", "TIME", "TITLE", "KCAL", "P", "C", "F"},
	}
	var totalKcal, totalP, totalC, totalF float64
	for _, m := range meals {
		tbl.Rows = append(tbl.Rows, []string{
			fmt.Sprintf("%d", m.ID),
			m.EatenAt.In(loc).Format("2006-01-02 15:04"),
			m.Title,
			fmt.Sprintf("%.0f", m.Kcal),
			fmt.Sprintf("%.0f", m.ProteinG),
			fmt.Sprintf("%.0f", m.CarbsG),
			fmt.Sprintf("%.0f", m.FatG),
		})
		totalKcal += m.Kcal
		totalP += m.ProteinG
		totalC += m.CarbsG
		totalF += m.FatG
	}
	tbl.Footer = []string{
		"", "", "total",
		fmt.Sprintf("%.0f", totalKcal),
		fmt.Sprintf("%.0f", totalP),
		fmt.Sprintf("%.0f", totalC),
		fmt.Sprintf("%.0f", totalF),
	}
	return Result{Table: tbl}, nil
}
