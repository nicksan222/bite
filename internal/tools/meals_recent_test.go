package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestMealsRecent_empty(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("meals_recent").Run(ctx, deps, NewArgs(map[string]any{
		"limit": float64(5),
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "no meals")
}

func TestMealsRecent_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	require.NoError(t, deps.Store.Close())
	_, err := MustGet("meals_recent").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}

func TestMealsRecent_returnsTable(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	for range 3 {
		_, err := deps.Store.SaveMeal(ctx, db.MealInput{
			Title: "M", Kcal: 100, EatenAt: deps.Now(),
		})
		require.NoError(t, err)
	}
	res, err := MustGet("meals_recent").Run(ctx, deps, NewArgs(map[string]any{
		"limit": float64(10),
	}))
	require.NoError(t, err)
	require.NotNil(t, res.Table)
	assert.Len(t, res.Table.Rows, 3)
	assert.Equal(t, []string{"ID", "TIME", "TITLE", "KCAL", "P", "C", "F"}, res.Table.Headers)
	assert.Equal(t, "300", res.Table.Footer[3])
}

func TestMealsRecent_zeroLimitFallsBackToDefault(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	for range 3 {
		_, err := deps.Store.SaveMeal(ctx, db.MealInput{
			Title: "M", Kcal: 100, EatenAt: deps.Now(),
		})
		require.NoError(t, err)
	}
	// limit=0 (and limit absent) should both use the default of 10.
	res, err := MustGet("meals_recent").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	require.NotNil(t, res.Table)
	assert.Len(t, res.Table.Rows, 3)

	res, err = MustGet("meals_recent").Run(ctx, deps, NewArgs(map[string]any{"limit": float64(0)}))
	require.NoError(t, err)
	require.NotNil(t, res.Table)
}
