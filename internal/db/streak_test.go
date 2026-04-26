package db

import (
	"context"
	"testing"
	"time"
)

// mealOnDay creates a Meal with EatenAt set to noon on the given date in UTC.
func mealOnDay(y int, m time.Month, d int) Meal {
	return Meal{EatenAt: time.Date(y, m, d, 12, 0, 0, 0, time.UTC)}
}

func TestComputeStreak_noMeals(t *testing.T) {
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	if got := computeStreak(nil, now, time.UTC); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestComputeStreak_todayOnly(t *testing.T) {
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	meals := []Meal{mealOnDay(2026, 4, 25)}
	if got := computeStreak(meals, now, time.UTC); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestComputeStreak_consecutiveDays(t *testing.T) {
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	meals := []Meal{
		mealOnDay(2026, 4, 23),
		mealOnDay(2026, 4, 24),
		mealOnDay(2026, 4, 25),
	}
	if got := computeStreak(meals, now, time.UTC); got != 3 {
		t.Errorf("got %d, want 3", got)
	}
}

func TestComputeStreak_gapBreaksStreak(t *testing.T) {
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	meals := []Meal{
		mealOnDay(2026, 4, 22), // gap on 23
		mealOnDay(2026, 4, 24),
		mealOnDay(2026, 4, 25),
	}
	if got := computeStreak(meals, now, time.UTC); got != 2 {
		t.Errorf("got %d, want 2 (streak since April 24)", got)
	}
}

func TestComputeStreak_todayNoMealsCountsFromYesterday(t *testing.T) {
	// Today has no meals yet — streak counts from yesterday backwards.
	now := time.Date(2026, 4, 25, 7, 0, 0, 0, time.UTC) // early morning
	meals := []Meal{
		mealOnDay(2026, 4, 23),
		mealOnDay(2026, 4, 24),
		// 25 = today, no meals yet
	}
	if got := computeStreak(meals, now, time.UTC); got != 2 {
		t.Errorf("got %d, want 2 (streak through yesterday)", got)
	}
}

func TestComputeStreak_todayNoMealsYesterdayAlsoEmpty(t *testing.T) {
	now := time.Date(2026, 4, 25, 7, 0, 0, 0, time.UTC)
	// Only a meal two days ago
	meals := []Meal{mealOnDay(2026, 4, 23)}
	// yesterday (24) is empty → streak = 0
	if got := computeStreak(meals, now, time.UTC); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

func TestComputeStreak_multipleMealsSameDay(t *testing.T) {
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	meals := []Meal{
		{EatenAt: time.Date(2026, 4, 25, 8, 0, 0, 0, time.UTC)},
		{EatenAt: time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)},
		{EatenAt: time.Date(2026, 4, 25, 19, 0, 0, 0, time.UTC)},
	}
	if got := computeStreak(meals, now, time.UTC); got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestComputeStreak_longStreak(t *testing.T) {
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)
	meals := make([]Meal, 30)
	for i := range meals {
		meals[i] = mealOnDay(2026, 3, 27+i) // march 27 – april 25 (30 days)
	}
	// Some days in April might overflow — use AddDate to be safe.
	base := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	for i := range meals {
		meals[i] = Meal{EatenAt: base.AddDate(0, 0, i)}
	}
	if got := computeStreak(meals, now, time.UTC); got != 30 {
		t.Errorf("got %d, want 30", got)
	}
}

// ─── Store.Streak integration tests ──────────────────────────────────────────

func TestStoreStreak_empty(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	streak, err := store.Streak(ctx, time.Now(), time.UTC)
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if streak != 0 {
		t.Errorf("expected 0 for empty store, got %d", streak)
	}
}

func TestStoreStreak_withMeals(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	now := time.Date(2026, 4, 25, 18, 0, 0, 0, time.UTC)

	for i := range 3 {
		day := time.Date(2026, 4, 23+i, 12, 0, 0, 0, time.UTC)
		_, _ = store.SaveMeal(ctx, MealInput{Title: "m", EatenAt: day})
	}

	streak, err := store.Streak(ctx, now, time.UTC)
	if err != nil {
		t.Fatalf("Streak: %v", err)
	}
	if streak != 3 {
		t.Errorf("expected streak 3, got %d", streak)
	}
}

func TestStoreStreak_nilLoc(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	// nil loc should not panic
	_, err := store.Streak(ctx, time.Now(), nil)
	if err != nil {
		t.Fatalf("Streak(nil loc): %v", err)
	}
}
