package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogMealFromMedia_savesAndReports(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: analysisJSON}

	res, err := MustGet("log_meal_from_media").Run(ctx, deps, NewArgs(map[string]any{
		"text": "200g pasta",
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "logged")
	assert.Contains(t, res.Text, "pasta")
	assert.Contains(t, res.Text, "550")

	meals, err := deps.Store.ListRecentMeals(ctx, 10)
	require.NoError(t, err)
	require.Len(t, meals, 1)
	assert.Equal(t, "pasta", meals[0].Title)
}

func TestLogMealFromMedia_emptyInput(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: analysisJSON}
	_, err := MustGet("log_meal_from_media").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}

func TestLogMealFromMedia_saveError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: analysisJSON}
	require.NoError(t, deps.Store.Close()) // forces SaveMeal to error after analyze succeeds

	_, err := MustGet("log_meal_from_media").Run(ctx, deps, NewArgs(map[string]any{
		"text": "lunch",
	}))
	require.Error(t, err)
}
