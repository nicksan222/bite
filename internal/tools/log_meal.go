package tools

import (
	"context"
	"fmt"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	Register(Tool{
		Name:    "log_meal",
		Summary: "Log a meal to the user's food diary.",
		Description: `Log a meal to the user's food diary. Estimate all nutritional fields you
can from the description; leave unknown fields at 0.`,
		Prompt: `Call log_meal whenever the user reports having eaten something — even casually
("had a salad", "just finished lunch"). Estimate kcal and macros from the
description; leave unknown fields at 0.`,
		Params: []Param{
			{Name: "title", Type: ParamString, Required: true, Positional: true,
				Desc: "Short meal name (e.g. 'Chicken salad', 'Pasta bolognese')."},
			{Name: "description", Type: ParamString,
				Desc: "Longer description of the meal (ingredients, portion size, preparation)."},
			{Name: "items", Type: ParamStringList,
				Desc: "Individual food items."},
			{Name: "kcal", Type: ParamFloat, Desc: "Estimated total kilocalories."},
			{Name: "protein_g", Type: ParamFloat, Desc: "Estimated protein in grams."},
			{Name: "carbs_g", Type: ParamFloat, Desc: "Estimated carbohydrates in grams."},
			{Name: "fat_g", Type: ParamFloat, Desc: "Estimated fat in grams."},
		},
		Run: runLogMeal,
	})
}

func runLogMeal(ctx context.Context, deps Deps, args Args) (Result, error) {
	meal, err := deps.Store.SaveMeal(ctx, db.MealInput{
		Title:       args.String("title"),
		Description: args.String("description"),
		Items:       args.StringList("items"),
		Kcal:        args.Float("kcal"),
		ProteinG:    args.Float("protein_g"),
		CarbsG:      args.Float("carbs_g"),
		FatG:        args.Float("fat_g"),
		EatenAt:     deps.NowOrDefault(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("save meal: %w", err)
	}
	return Result{
		Text: fmt.Sprintf(
			"meal logged (id=%d): %s — %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
			meal.ID, meal.Title, meal.Kcal, meal.ProteinG, meal.CarbsG, meal.FatG,
		),
	}, nil
}
