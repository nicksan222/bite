package cli

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

// saveMealRecorder records calls to SaveMeal; embed mockStore for the rest.
type saveMealRecorder struct {
	mockStore
	saved []db.MealInput
	err   error
}

func (s *saveMealRecorder) SaveMeal(_ context.Context, in db.MealInput) (db.Meal, error) {
	s.saved = append(s.saved, in)
	if s.err != nil {
		return db.Meal{}, s.err
	}
	return db.Meal{
		ID:       1,
		Title:    in.Title,
		Kcal:     in.Kcal,
		ProteinG: in.ProteinG,
		CarbsG:   in.CarbsG,
		FatG:     in.FatG,
	}, nil
}

// openTestStore opens a real :memory: SQLite store for integration tests.
func openTestStore(t *testing.T) db.Storer {
	t.Helper()
	store, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err, "db.Open")
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// ─── logMealTool ─────────────────────────────────────────────────────────────

func TestLogMealTool_name(t *testing.T) {
	assert.Equal(t, "log_meal", logMealTool(&mockStore{}).Name)
}

func TestLogMealTool_hasParameters(t *testing.T) {
	tool := logMealTool(&mockStore{})
	assert.NotEmpty(t, tool.Parameters)
	assert.Contains(t, tool.Parameters, "title")
}

func TestLogMealTool_execute_invalidJSON(t *testing.T) {
	_, err := logMealTool(&mockStore{}).Execute(context.Background(), "not json")
	assert.Error(t, err)
}

func TestLogMealTool_execute_storeError(t *testing.T) {
	rec := &saveMealRecorder{err: errors.New("disk full")}
	args, _ := json.Marshal(map[string]any{"title": "Salad"})
	_, err := logMealTool(rec).Execute(context.Background(), string(args))
	require.Error(t, err)
	assert.ErrorContains(t, err, "disk full")
}

func TestLogMealTool_execute_missingTitle_storeErrors(t *testing.T) {
	rec := &saveMealRecorder{err: errors.New("title is required")}
	args, _ := json.Marshal(map[string]any{"title": ""})
	_, err := logMealTool(rec).Execute(context.Background(), string(args))
	assert.Error(t, err)
}

func TestLogMealTool_minimalArgs(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	args, _ := json.Marshal(map[string]any{"title": "Apple"})
	result, err := logMealTool(store).Execute(ctx, string(args))
	require.NoError(t, err)
	assert.Contains(t, result, "Apple")
	assert.Contains(t, result, "meal logged")
}

// ─── getMealsTodayTool ────────────────────────────────────────────────────────

func TestGetMealsTodayTool_noParameters(t *testing.T) {
	assert.Empty(t, getMealsTodayTool(newMockStore()).Parameters)
}

func TestGetMealsTodayTool_storeError(t *testing.T) {
	store := newMockStore()
	store.listMealsErr = errors.New("db offline")
	_, err := getMealsTodayTool(store).Execute(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "db offline")
}

func TestGetMealsTodayTool_noMeals(t *testing.T) {
	result, err := getMealsTodayTool(openTestStore(t)).Execute(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "No meals logged today.", result)
}

func TestLogThenGetToday_roundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	args, _ := json.Marshal(map[string]any{
		"title": "Grilled chicken", "kcal": 450, "protein_g": 55, "carbs_g": 5, "fat_g": 20,
	})
	logResult, err := logMealTool(store).Execute(ctx, string(args))
	require.NoError(t, err)
	assert.Contains(t, logResult, "meal logged")

	getResult, err := getMealsTodayTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, getResult, "Grilled chicken")
	assert.Contains(t, getResult, "450")
}

// ─── getMealsWeekTool ─────────────────────────────────────────────────────────

func TestGetMealsWeekTool_noParameters(t *testing.T) {
	assert.Empty(t, getMealsWeekTool(newMockStore()).Parameters)
}

func TestGetMealsWeekTool_storeError(t *testing.T) {
	store := newMockStore()
	store.listMealsErr = errors.New("db offline")
	_, err := getMealsWeekTool(store).Execute(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "db offline")
}

func TestGetMealsWeekTool_noMeals(t *testing.T) {
	result, err := getMealsWeekTool(openTestStore(t)).Execute(context.Background(), "")
	require.NoError(t, err)
	assert.Equal(t, "No meals logged this week.", result)
}

func TestLogThenGetWeek_roundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	args, _ := json.Marshal(map[string]any{"title": "Oatmeal", "kcal": 300})
	_, err := logMealTool(store).Execute(ctx, string(args))
	require.NoError(t, err)

	result, err := getMealsWeekTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, result, "Oatmeal")
	assert.Contains(t, result, "Week total")
}

// ─── setGoalsTool ─────────────────────────────────────────────────────────────

func TestSetGoalsTool_name(t *testing.T) {
	assert.Equal(t, "set_goals", setGoalsTool(&mockStore{}).Name)
}

func TestSetGoalsTool_invalidJSON(t *testing.T) {
	_, err := setGoalsTool(&mockStore{}).Execute(context.Background(), "not json")
	assert.Error(t, err)
}

func TestSetGoalsTool_getGoalsError(t *testing.T) {
	store := newMockStore()
	store.goalsErr = errors.New("db offline")
	args, _ := json.Marshal(map[string]any{"kcal": 2000})
	_, err := setGoalsTool(store).Execute(context.Background(), string(args))
	require.Error(t, err)
	assert.ErrorContains(t, err, "db offline")
}

func TestSetGoalsTool_setGoalsError(t *testing.T) {
	store := newMockStore()
	store.setGoalsErr = errors.New("write failed")
	args, _ := json.Marshal(map[string]any{"kcal": 2000})
	_, err := setGoalsTool(store).Execute(context.Background(), string(args))
	require.Error(t, err)
	assert.ErrorContains(t, err, "write failed")
}

func TestSetThenGetGoals_roundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	args, _ := json.Marshal(map[string]any{"kcal": 2000, "protein_g": 150})
	setResult, err := setGoalsTool(store).Execute(ctx, string(args))
	require.NoError(t, err)
	assert.Contains(t, setResult, "2000")

	getResult, err := getGoalsTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, getResult, "2000")
	assert.Contains(t, getResult, "150")
}

func TestSetGoalsTool_clearKcal(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	args, _ := json.Marshal(map[string]any{"kcal": 2000})
	_, err := setGoalsTool(store).Execute(ctx, string(args))
	require.NoError(t, err)

	clearArgs, _ := json.Marshal(map[string]any{"kcal": 0})
	result, err := setGoalsTool(store).Execute(ctx, string(clearArgs))
	require.NoError(t, err)
	assert.Contains(t, result, "not set")
}

func TestSetGoalsTool_preservesUnmentioned(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	args, _ := json.Marshal(map[string]any{"protein_g": 150})
	_, err := setGoalsTool(store).Execute(ctx, string(args))
	require.NoError(t, err)

	args2, _ := json.Marshal(map[string]any{"kcal": 1800})
	_, err = setGoalsTool(store).Execute(ctx, string(args2))
	require.NoError(t, err)

	result, err := getGoalsTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, result, "150")
	assert.Contains(t, result, "1800")
}

// ─── getGoalsTool ─────────────────────────────────────────────────────────────

func TestGetGoalsTool_noParameters(t *testing.T) {
	assert.Empty(t, getGoalsTool(newMockStore()).Parameters)
}

func TestGetGoalsTool_storeError(t *testing.T) {
	store := newMockStore()
	store.goalsErr = errors.New("db offline")
	_, err := getGoalsTool(store).Execute(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "db offline")
}

func TestGetGoalsTool_noGoals(t *testing.T) {
	result, err := getGoalsTool(openTestStore(t)).Execute(context.Background(), "")
	require.NoError(t, err)
	assert.Contains(t, result, "not set")
}

// ─── deleteMealTool ───────────────────────────────────────────────────────────

func TestDeleteMealTool_name(t *testing.T) {
	assert.Equal(t, "delete_meal", deleteMealTool(&mockStore{}).Name)
}

func TestDeleteMealTool_hasParameters(t *testing.T) {
	assert.Contains(t, deleteMealTool(&mockStore{}).Parameters, "meal_id")
}

func TestDeleteMealTool_invalidJSON(t *testing.T) {
	_, err := deleteMealTool(&mockStore{}).Execute(context.Background(), "not json")
	assert.Error(t, err)
}

func TestDeleteMealTool_getMealError(t *testing.T) {
	store := newMockStore()
	store.getMealErr = errors.New("db offline")
	args, _ := json.Marshal(map[string]any{"meal_id": 1})
	_, err := deleteMealTool(store).Execute(context.Background(), string(args))
	require.Error(t, err)
	assert.ErrorContains(t, err, "db offline")
}

func TestDeleteMealTool_deleteError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	meal, _ := store.SaveMeal(ctx, db.MealInput{Title: "x", EatenAt: time.Now()})
	store.deleteMealErr = errors.New("disk full")
	args, _ := json.Marshal(map[string]any{"meal_id": meal.ID})
	_, err := deleteMealTool(store).Execute(ctx, string(args))
	require.Error(t, err)
	assert.ErrorContains(t, err, "disk full")
}

func TestDeleteMealTool_notFound(t *testing.T) {
	args, _ := json.Marshal(map[string]any{"meal_id": 9999})
	_, err := deleteMealTool(openTestStore(t)).Execute(context.Background(), string(args))
	assert.Error(t, err)
}

func TestLogThenDelete_roundTrip(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)

	logArgs, _ := json.Marshal(map[string]any{"title": "Pizza", "kcal": 800})
	_, err := logMealTool(store).Execute(ctx, string(logArgs))
	require.NoError(t, err)

	meals, err := store.ListRecentMeals(ctx, 1)
	require.NoError(t, err)
	require.Len(t, meals, 1)

	delArgs, _ := json.Marshal(map[string]any{"meal_id": meals[0].ID})
	delResult, err := deleteMealTool(store).Execute(ctx, string(delArgs))
	require.NoError(t, err)
	assert.Contains(t, delResult, "deleted")

	getResult, err := getMealsTodayTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Equal(t, "No meals logged today.", getResult)
}

// ─── getStreakTool ────────────────────────────────────────────────────────────

func TestGetStreakTool_noParameters(t *testing.T) {
	assert.Empty(t, getStreakTool(newMockStore()).Parameters)
}

func TestGetStreakTool_storeError(t *testing.T) {
	store := newMockStore()
	store.streakErr = errors.New("db offline")
	_, err := getStreakTool(store).Execute(context.Background(), "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "db offline")
}

func TestGetStreakTool_noStreak(t *testing.T) {
	result, err := getStreakTool(openTestStore(t)).Execute(context.Background(), "")
	require.NoError(t, err)
	assert.Contains(t, result, "No active streak")
}

func TestGetStreakTool_oneDay(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 12, 0, 0, 0, time.UTC)
	_, err := store.SaveMeal(ctx, db.MealInput{Title: "breakfast", EatenAt: today})
	require.NoError(t, err)

	result, err := getStreakTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, result, "1 day")
}

func TestGetStreak_withRealStore(t *testing.T) {
	ctx := context.Background()
	store := openTestStore(t)
	now := time.Now()
	for i := 2; i >= 0; i-- {
		day := time.Date(now.Year(), now.Month(), now.Day()-i, 12, 0, 0, 0, time.UTC)
		_, err := store.SaveMeal(ctx, db.MealInput{Title: "meal", EatenAt: day})
		require.NoError(t, err)
	}

	result, err := getStreakTool(store).Execute(ctx, "")
	require.NoError(t, err)
	assert.Contains(t, result, "3 days")
}

// ─── makeChatTools ────────────────────────────────────────────────────────────

func TestMakeChatTools_returnsNonEmpty(t *testing.T) {
	assert.NotEmpty(t, makeChatTools(&mockStore{}))
}

func TestMakeChatTools_allToolsValid(t *testing.T) {
	tools := makeChatTools(&mockStore{})
	seen := make(map[string]bool, len(tools))
	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name, "tool has empty name")
		assert.False(t, seen[tool.Name], "duplicate tool name: %q", tool.Name)
		seen[tool.Name] = true
		assert.NotNil(t, tool.Execute, "tool %q has nil Execute", tool.Name)
		assert.NotEmpty(t, tool.Description, "tool %q has empty description", tool.Name)
	}
}
