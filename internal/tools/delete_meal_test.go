package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestDeleteMeal_removes(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	meal, err := deps.Store.SaveMeal(ctx, db.MealInput{Title: "X", Kcal: 100, EatenAt: deps.Now()})
	require.NoError(t, err)

	res, err := MustGet("delete_meal").Run(ctx, deps, NewArgs(map[string]any{
		"meal_id": float64(meal.ID),
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "deleted")
	assert.Contains(t, res.Text, "X")

	got, _ := deps.Store.ListRecentMeals(ctx, 10)
	assert.Empty(t, got)
}

func TestDeleteMeal_missingID(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := MustGet("delete_meal").Run(ctx, deps, NewArgs(map[string]any{
		"meal_id": float64(999),
	}))
	require.Error(t, err)
}

func TestDeleteMeal_storeDeleteError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	meal, err := deps.Store.SaveMeal(ctx, db.MealInput{Title: "X", Kcal: 100, EatenAt: deps.Now()})
	require.NoError(t, err)
	require.NoError(t, deps.Store.Close()) // closing forces both GetMeal and DeleteMeal to error

	_, err = MustGet("delete_meal").Run(ctx, deps, NewArgs(map[string]any{
		"meal_id": float64(meal.ID),
	}))
	require.Error(t, err)
}
