package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestMealsToday_empty(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("meals_today").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "No meals logged today")
}

func TestMealsToday_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	require.NoError(t, deps.Store.Close()) // forces ListMealsForDay to error

	_, err := MustGet("meals_today").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}

func TestMealsToday_summarises(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := deps.Store.SaveMeal(ctx, db.MealInput{
		Title: "Eggs", Kcal: 200, ProteinG: 18, EatenAt: deps.Now(),
	})
	require.NoError(t, err)
	_, err = deps.Store.SaveMeal(ctx, db.MealInput{
		Title: "Toast", Kcal: 150, CarbsG: 30, EatenAt: deps.Now(),
	})
	require.NoError(t, err)

	res, err := MustGet("meals_today").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "Eggs")
	assert.Contains(t, res.Text, "Toast")
	assert.Contains(t, res.Text, "350") // 200+150
	assert.Contains(t, res.Text, "Total")
}
