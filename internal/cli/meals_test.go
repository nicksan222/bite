package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestShowMeals_empty(t *testing.T) {
	store := newMockStore()
	var buf bytes.Buffer
	err := showMeals(context.Background(), &buf, store, 0, time.Now(), time.UTC)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "no meals logged")
}

func TestShowMeals_today(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	_, err := store.SaveMeal(ctx, db.MealInput{Title: "lunch", Kcal: 600, ProteinG: 40, CarbsG: 70, FatG: 15, EatenAt: now})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, showMeals(ctx, &buf, store, 0, now, time.UTC))
	out := buf.String()
	assert.Contains(t, out, "lunch")
	assert.Contains(t, out, "600")
	assert.Contains(t, out, "total:")
}

func TestShowMeals_recent(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	base := time.Date(2026, 4, 25, 8, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		_, _ = store.SaveMeal(ctx, db.MealInput{
			Title:   "meal",
			Kcal:    100,
			EatenAt: base.Add(time.Duration(i) * time.Hour),
		})
	}

	var buf bytes.Buffer
	require.NoError(t, showMeals(ctx, &buf, store, 3, base, time.UTC))
	assert.Contains(t, buf.String(), "total:")
}

func TestShowMeals_nilLoc(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	now := time.Now()
	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "snack", EatenAt: now})

	var buf bytes.Buffer
	// nil loc should not panic
	require.NoError(t, showMeals(ctx, &buf, store, 0, now, nil))
}

func TestShowMeals_storeError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	store.listMealsErr = errors.New("db offline")

	err := showMeals(ctx, &bytes.Buffer{}, store, 0, time.Now(), time.UTC)
	require.Error(t, err)
}

func TestShowMeals_recentStoreError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	store.listMealsErr = errors.New("db offline")

	err := showMeals(ctx, &bytes.Buffer{}, store, 5, time.Now(), time.UTC)
	require.Error(t, err)
}

func TestRunMeals_integration(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Int64P("recent", "n", 0, "recent meals")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runMeals(cmd, nil))
	assert.Contains(t, buf.String(), "no meals")
}

func TestRunMeals_dateFlag(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Int64P("recent", "n", 0, "recent meals")
	cmd.Flags().StringP("date", "d", "", "date")
	_ = cmd.Flags().Set("date", "2026-04-20")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runMeals(cmd, nil))
	// no meals logged for that date — expect "no meals" message
	assert.Contains(t, buf.String(), "no meals")
}

func TestRunMeals_invalidDate(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Int64P("recent", "n", 0, "recent meals")
	cmd.Flags().StringP("date", "d", "", "date")
	_ = cmd.Flags().Set("date", "not-a-date")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runMeals(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YYYY-MM-DD")
}

// ─── deleteMeal helper tests ──────────────────────────────────────────────────

func TestDeleteMeal_success(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	meal, _ := store.SaveMeal(ctx, db.MealInput{Title: "pasta", Kcal: 500, EatenAt: now})

	var buf bytes.Buffer
	require.NoError(t, deleteMeal(ctx, &buf, store, meal.ID))
	assert.Contains(t, buf.String(), "pasta")
	assert.Contains(t, buf.String(), "deleted")
	// Verify it's gone from the store.
	meals, _ := store.ListRecentMeals(ctx, 10)
	assert.Len(t, meals, 0)
}

func TestDeleteMeal_notFound(t *testing.T) {
	store := newMockStore()
	err := deleteMeal(context.Background(), &bytes.Buffer{}, store, 9999)
	require.Error(t, err)
}

func TestDeleteMeal_getMealError(t *testing.T) {
	store := newMockStore()
	store.getMealErr = errors.New("db offline")
	err := deleteMeal(context.Background(), &bytes.Buffer{}, store, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db offline")
}

func TestDeleteMeal_deleteError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	meal, _ := store.SaveMeal(ctx, db.MealInput{Title: "x", EatenAt: time.Now()})
	store.deleteMealErr = errors.New("disk full")
	err := deleteMeal(ctx, &bytes.Buffer{}, store, meal.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestRunDeleteMeal_integration(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	// runDeleteMeal tries to delete meal ID 9999 — store is empty so GetMeal fails.
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runDeleteMeal(cmd, []string{"9999"})
	require.Error(t, err)
}

func TestRunDeleteMeal_invalidID(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runDeleteMeal(cmd, []string{"not-a-number"})
	require.Error(t, err)
}

// ─── weekStart helper ─────────────────────────────────────────────────────────

func TestWeekStart_monday(t *testing.T) {
	// 2026-04-27 is a Monday — weekStart should return that same day at midnight.
	mon := time.Date(2026, 4, 27, 14, 30, 0, 0, time.UTC)
	got := weekStart(mon, time.UTC)
	want := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

func TestWeekStart_wednesday(t *testing.T) {
	wed := time.Date(2026, 4, 29, 9, 0, 0, 0, time.UTC)
	got := weekStart(wed, time.UTC)
	want := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

func TestWeekStart_sunday(t *testing.T) {
	sun := time.Date(2026, 5, 3, 23, 59, 0, 0, time.UTC)
	got := weekStart(sun, time.UTC)
	want := time.Date(2026, 4, 27, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

// ─── showMealsWeek tests ──────────────────────────────────────────────────────

func TestShowMealsWeek_empty(t *testing.T) {
	store := newMockStore()
	var buf bytes.Buffer
	err := showMealsWeek(context.Background(), &buf, store, time.Now(), time.UTC)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "no meals logged this week")
}

func TestShowMealsWeek_withMeals(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	// Monday 2026-04-27
	mon := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)
	// Wednesday 2026-04-29
	wed := time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC)
	// Use any time in the same week as the reference
	now := time.Date(2026, 4, 29, 20, 0, 0, 0, time.UTC)

	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "Breakfast", Kcal: 400, ProteinG: 20, EatenAt: mon})
	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "Lunch", Kcal: 600, ProteinG: 40, EatenAt: wed})

	var buf bytes.Buffer
	require.NoError(t, showMealsWeek(ctx, &buf, store, now, time.UTC))
	out := buf.String()
	for _, want := range []string{"Breakfast", "Lunch", "1000", "week total"} {
		assert.Contains(t, out, want)
	}
}

func TestShowMealsWeek_nilLoc(t *testing.T) {
	store := newMockStore()
	var buf bytes.Buffer
	require.NoError(t, showMealsWeek(context.Background(), &buf, store, time.Now(), nil))
}

func TestShowMealsWeek_storeError(t *testing.T) {
	store := newMockStore()
	store.listMealsErr = errors.New("db offline")
	err := showMealsWeek(context.Background(), &bytes.Buffer{}, store, time.Now(), time.UTC)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db offline")
}

func TestShowMealsWeek_flushError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	now := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "x", Kcal: 100, EatenAt: now})
	err := showMealsWeek(ctx, failWriter{}, store, now, time.UTC)
	require.Error(t, err)
}

func TestRunMeals_weekFlag(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Int64P("recent", "n", 0, "recent")
	cmd.Flags().StringP("date", "d", "", "date")
	cmd.Flags().BoolP("week", "w", false, "week")
	_ = cmd.Flags().Set("week", "true")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runMeals(cmd, nil))
	assert.Contains(t, buf.String(), "no meals logged this week")
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestShowMeals_flushError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "x", Kcal: 100, EatenAt: now})

	// failWriter causes tabwriter.Flush to return an error.
	err := showMeals(ctx, failWriter{}, store, 0, now, time.UTC)
	require.Error(t, err)
}

func TestShowMeals_totals(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "a", Kcal: 100, ProteinG: 10, CarbsG: 20, FatG: 5, EatenAt: now})
	_, _ = store.SaveMeal(ctx, db.MealInput{Title: "b", Kcal: 200, ProteinG: 20, CarbsG: 30, FatG: 10, EatenAt: now})

	var buf bytes.Buffer
	require.NoError(t, showMeals(ctx, &buf, store, 0, now, time.UTC))
	// 100+200=300 kcal total
	assert.Contains(t, buf.String(), "300")
}
