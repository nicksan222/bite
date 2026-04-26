package tools

import (
	"context"
	"fmt"
)

func init() {
	Register(Tool{
		Name:        "delete_meal",
		Summary:     "Delete a logged meal by ID.",
		Description: `Delete a previously logged meal by its ID.`,
		Prompt: `Call delete_meal when the user wants to remove a meal entry they logged by
mistake or wants to undo a recent log. Always confirm with the user (and
show the meal you are about to delete) before calling.`,
		Params: []Param{
			{Name: "meal_id", Type: ParamInt, Required: true, Positional: true,
				Desc: "ID of the meal to delete (shown in the meals list)."},
		},
		Run: runDeleteMeal,
	})
}

func runDeleteMeal(ctx context.Context, deps Deps, args Args) (Result, error) {
	id := args.Int("meal_id")
	meal, err := deps.Store.GetMeal(ctx, id)
	if err != nil {
		return Result{}, fmt.Errorf("meal %d not found: %w", id, err)
	}
	if err := deps.Store.DeleteMeal(ctx, id); err != nil {
		return Result{}, fmt.Errorf("delete meal: %w", err)
	}
	return Result{Text: fmt.Sprintf("deleted meal #%d: %s (%.0f kcal)", meal.ID, meal.Title, meal.Kcal)}, nil
}
