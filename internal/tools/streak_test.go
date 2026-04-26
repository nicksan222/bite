package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestStreak_zero(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("streak").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "No active streak")
}

func TestStreak_one(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := deps.Store.SaveMeal(ctx, db.MealInput{Title: "M", Kcal: 100, EatenAt: deps.Now()})
	require.NoError(t, err)

	res, err := MustGet("streak").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "1 day")
}

func TestStreak_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	require.NoError(t, deps.Store.Close()) // forces Streak() to error
	_, err := MustGet("streak").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}

func TestStreak_many(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	for i := range 3 {
		_, err := deps.Store.SaveMeal(ctx, db.MealInput{
			Title: "M", Kcal: 100, EatenAt: deps.Now().AddDate(0, 0, -i),
		})
		require.NoError(t, err)
	}
	res, err := MustGet("streak").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "3 days")
}
