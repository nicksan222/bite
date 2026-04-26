package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestMealsWeek_empty(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("meals_week").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "No meals logged this week")
}

func TestMealsWeek_groupsByDay(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	monday := weekStart(deps.Now(), time.UTC)

	_, err := deps.Store.SaveMeal(ctx, db.MealInput{Title: "M1", Kcal: 100, EatenAt: monday})
	require.NoError(t, err)
	_, err = deps.Store.SaveMeal(ctx, db.MealInput{Title: "M2", Kcal: 200, EatenAt: monday})
	require.NoError(t, err)
	_, err = deps.Store.SaveMeal(ctx, db.MealInput{Title: "T1", Kcal: 300, EatenAt: monday.AddDate(0, 0, 1)})
	require.NoError(t, err)

	res, err := MustGet("meals_week").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "M1")
	assert.Contains(t, res.Text, "T1")
	assert.Contains(t, res.Text, "Week total")
	assert.Contains(t, res.Text, "600") // 100+200+300
}

func TestWeekStart_isMonday(t *testing.T) {
	loc := time.UTC
	wed := time.Date(2026, 4, 22, 12, 0, 0, 0, loc) // Wed
	got := weekStart(wed, loc)
	assert.Equal(t, time.Monday, got.Weekday())
	assert.Equal(t, time.Date(2026, 4, 20, 0, 0, 0, 0, loc), got)
}

func TestWeekStart_sundayRollsBack(t *testing.T) {
	loc := time.UTC
	sun := time.Date(2026, 4, 26, 12, 0, 0, 0, loc) // Sun
	got := weekStart(sun, loc)
	assert.Equal(t, time.Monday, got.Weekday())
	assert.Equal(t, time.Date(2026, 4, 20, 0, 0, 0, 0, loc), got)
}
