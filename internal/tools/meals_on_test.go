package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestMealsOn_empty(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("meals_on").Run(ctx, deps, NewArgs(map[string]any{
		"date": "2026-04-26",
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "no meals logged on 2026-04-26")
}

func TestMealsOn_filtersByDate(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	day := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	other := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	_, err := deps.Store.SaveMeal(ctx, db.MealInput{Title: "today", Kcal: 200, EatenAt: day})
	require.NoError(t, err)
	_, err = deps.Store.SaveMeal(ctx, db.MealInput{Title: "skip", Kcal: 999, EatenAt: other})
	require.NoError(t, err)

	res, err := MustGet("meals_on").Run(ctx, deps, NewArgs(map[string]any{
		"date": "2026-04-26",
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "today")
	assert.NotContains(t, res.Text, "skip")
	assert.Contains(t, res.Text, "200")
}

func TestMealsOn_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	require.NoError(t, deps.Store.Close())
	_, err := MustGet("meals_on").Run(ctx, deps, NewArgs(map[string]any{
		"date": "2026-04-26",
	}))
	require.Error(t, err)
}

func TestMealsOn_invalidDate(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := MustGet("meals_on").Run(ctx, deps, NewArgs(map[string]any{
		"date": "not-a-date",
	}))
	require.Error(t, err)
}
