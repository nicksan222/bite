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

func TestLogMeal_allZeroNutrition_rejected(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"title":       "lasagna",
		"description": "100g of lasagna",
	}))
	require.Error(t, err, "log_meal must refuse a row with no kcal and no macros")
	assert.Contains(t, err.Error(), "Estimate", "error should tell the model how to fix it")

	meals, err := deps.Store.ListRecentMeals(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, meals, "rejected meal must not be persisted")
}

func TestLogMeal_negativeNutritionAlsoRejected(t *testing.T) {
	// Defensive: negative numbers are nonsense and shouldn't slip past the
	// guardrail just because they're "non-zero".
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"title":     "weird",
		"kcal":      -100.0,
		"protein_g": -5.0,
	}))
	require.Error(t, err)
}

func TestLogMeal_singleMacroIsEnough(t *testing.T) {
	// One non-zero macro is still real information — the guardrail only
	// fires when literally everything is zero.
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"title":     "protein shake",
		"protein_g": 25.0,
	}))
	require.NoError(t, err)
}

func TestLogMeal_eatenAtUsesDepsNow(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	_, err := MustGet("log_meal").Run(ctx, deps, NewArgs(map[string]any{
		"title": "Snack",
		"kcal":  120.0,
	}))
	require.NoError(t, err)
	meals, err := deps.Store.ListRecentMeals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, meals, 1)
	assert.Equal(t, deps.Now().Unix(), meals[0].EatenAt.Unix())
}
