package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogMeal_savesAndReports(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	res, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"title":     "Pasta",
		"kcal":      550.0,
		"protein_g": 18.0,
		"carbs_g":   80.0,
		"fat_g":     12.0,
		"items":     []any{"pasta", "pesto"},
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "logged")
	assert.Contains(t, res.Text, "Pasta")
	assert.Contains(t, res.Text, "550")

	meals, err := deps.Store.ListRecentMeals(ctx, 10)
	require.NoError(t, err)
	require.Len(t, meals, 1)
	assert.Equal(t, "Pasta", meals[0].Title)
	assert.InDelta(t, 18.0, meals[0].ProteinG, 0.01)
}

func TestLogMeal_emptyTitle_errors(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"kcal": 100.0,
	}))
	require.Error(t, err)
}

func TestLogMeal_eatenAtUsesDepsNow(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"title": "Snack",
	}))
	require.NoError(t, err)
	meals, err := deps.Store.ListRecentMeals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, meals, 1)
	assert.Equal(t, deps.Now().Unix(), meals[0].EatenAt.Unix())
}
