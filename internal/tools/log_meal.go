package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	Register(Tool{
		Name:    "log_meal",
		Summary: "Log a meal to the user's food diary.",
		Description: `Log a meal to the user's food diary. Estimate kcal and macros from the
description — at least one of kcal/protein/carbs/fat must be greater than zero,
otherwise the entry is rejected.`,
		Prompt: `Call log_meal whenever the user reports having eaten something — even casually
("had a salad", "just finished lunch", "ate 100g of lasagna").

NEVER call log_meal with all zero values. Use your own nutrition knowledge to
estimate kcal and macros from the food and portion size described. If the
user gives a portion ("100g of lasagna"), scale to it; otherwise assume a
standard serving. A rough estimate is always better than zero — if you are
uncertain, estimate first and tell the user the assumption you made.`,
		Examples: []Example{
			{Cmd: `bite log_meal "200g pasta with pesto" --kcal 520 --protein_g 14 --carbs_g 60 --fat_g 24`, Desc: "log a meal from text"},
		},
		Params: []Param{
			{Name: "title", Type: ParamString, Required: true, Positional: true,
				Desc: "Short meal name (e.g. 'Chicken salad', 'Pasta bolognese')."},
			{Name: "description", Type: ParamString,
				Desc: "Longer description of the meal (ingredients, portion size, preparation)."},
			{Name: "items", Type: ParamStringList,
				Desc: "Individual food items."},
			{Name: "kcal", Type: ParamFloat, Desc: "Estimated total kilocalories. Required (must be > 0)."},
			{Name: "protein_g", Type: ParamFloat, Desc: "Estimated protein in grams."},
			{Name: "carbs_g", Type: ParamFloat, Desc: "Estimated carbohydrates in grams."},
			{Name: "fat_g", Type: ParamFloat, Desc: "Estimated fat in grams."},
		},
		Run: runLogMeal,
	})
}

// errEmptyMealNutrition is returned when a meal-logging tool would create a
// row with no nutritional information. The wording is shaped for the model:
// it tells the caller exactly what to do next, so a tool-result reply nudges
// the model into retrying with estimates instead of giving up. Smaller local
// models (e.g. ollama) frequently call log_meal with zeros — this guardrail
// forces a second pass with real numbers.
var errEmptyMealNutrition = errors.New(
	"log_meal rejected: kcal and all macros are zero. Estimate kcal " +
		"(and ideally protein_g/carbs_g/fat_g) from typical nutrition data " +
		"for the food and portion described, then retry. Never log a meal " +
		"with all zeros.",
)

// requireMealNutrition is the shared guardrail for the meal-logging tools.
// It rejects any row whose kcal and three macros are all <= 0 (zero or
// negative — both meaning "no real data"). The check lives here rather than
// in db.Store because direct/CLI callers may legitimately log a snack
// without macros; only the tool surface (where the model can blunder) needs
// this floor.
func requireMealNutrition(kcal, protein, carbs, fat float64) error {
	if kcal <= 0 && protein <= 0 && carbs <= 0 && fat <= 0 {
		return errEmptyMealNutrition
	}
	return nil
}

func runLogMeal(ctx context.Context, deps Deps, args Args) (Result, error) {
	kcal := args.Float("kcal")
	protein := args.Float("protein_g")
	carbs := args.Float("carbs_g")
	fat := args.Float("fat_g")
	if err := requireMealNutrition(kcal, protein, carbs, fat); err != nil {
		return Result{}, err
	}

	meal, err := deps.Store.SaveMeal(ctx, db.MealInput{
		Title:       args.String("title"),
		Description: args.String("description"),
		Items:       args.StringList("items"),
		Kcal:        kcal,
		ProteinG:    protein,
		CarbsG:      carbs,
		FatG:        fat,
		EatenAt:     deps.NowOrDefault(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("save meal: %w", err)
	}
	return Result{Text: formatMealSaved(meal)}, nil
}
