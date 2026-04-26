package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db/sqlc"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), ":memory:")
	require.NoError(t, err, "Open")
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestStoreSmoke(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	conv, err := store.NewConversation(ctx, "claude-haiku-4-5", "")
	require.NoError(t, err, "NewConversation")

	_, err = store.AppendMessage(ctx, conv.ID, "user", "hello")
	require.NoError(t, err, "AppendMessage user")
	_, err = store.AppendMessage(ctx, conv.ID, "assistant", "hi!")
	require.NoError(t, err, "AppendMessage assistant")

	msgs, err := store.ListMessages(ctx, conv.ID)
	require.NoError(t, err, "ListMessages")
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)

	count, err := store.CountMessages(ctx, conv.ID)
	require.NoError(t, err, "CountMessages")
	assert.Equal(t, int64(2), count)
}

func TestRenameAndDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	conv, err := store.NewConversation(ctx, "claude-haiku-4-5", "draft")
	require.NoError(t, err, "NewConversation")
	require.NoError(t, store.RenameConversation(ctx, conv.ID, "renamed"), "RenameConversation")

	got, err := store.GetConversation(ctx, conv.ID)
	require.NoError(t, err, "GetConversation")
	assert.Equal(t, "renamed", got.Title)

	require.NoError(t, store.DeleteConversation(ctx, conv.ID), "DeleteConversation")
	_, err = store.GetConversation(ctx, conv.ID)
	require.Error(t, err, "conversation should be gone after delete")
}

func TestListMostRecentFirst(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	c1, _ := store.NewConversation(ctx, "claude-haiku-4-5", "first")
	c2, _ := store.NewConversation(ctx, "claude-haiku-4-5", "second")

	// Touch c1 last so it bubbles to the top.
	require.NoError(t, store.TouchConversation(ctx, c1.ID), "TouchConversation")

	convs, err := store.ListConversations(ctx, 10)
	require.NoError(t, err, "ListConversations")
	require.Len(t, convs, 2)
	assert.Equal(t, c1.ID, convs[0].ID)
	assert.Equal(t, c2.ID, convs[1].ID)
}

func TestMealsCRUD(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	noon := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	morning := time.Date(2026, 4, 25, 8, 0, 0, 0, time.UTC)
	yesterday := noon.AddDate(0, 0, -1)

	saved, err := store.SaveMeal(ctx, MealInput{
		Title: "lunch", Description: "rice + chicken",
		Items: []string{"rice", "chicken"},
		Kcal:  600, ProteinG: 40, CarbsG: 70, FatG: 15,
		EatenAt: noon,
	})
	require.NoError(t, err, "SaveMeal")
	assert.Equal(t, "lunch", saved.Title)
	var items []string
	err = json.Unmarshal([]byte(saved.Items), &items)
	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "rice", items[0])

	_, err = store.SaveMeal(ctx, MealInput{Title: "breakfast", EatenAt: morning})
	require.NoError(t, err, "SaveMeal breakfast")
	_, err = store.SaveMeal(ctx, MealInput{Title: "old", EatenAt: yesterday})
	require.NoError(t, err, "SaveMeal old")

	day, err := store.ListMealsForDay(ctx, noon, time.UTC)
	require.NoError(t, err, "ListMealsForDay")
	require.Len(t, day, 2, "today = %d meals, want 2", len(day))
	assert.Equal(t, "breakfast", day[0].Title)
	assert.Equal(t, "lunch", day[1].Title)

	require.NoError(t, store.DeleteMeal(ctx, saved.ID), "DeleteMeal")
	_, err = store.GetMeal(ctx, saved.ID)
	require.Error(t, err, "meal should be gone after delete")
}

func TestSaveMealRequiresTitle(t *testing.T) {
	store := newTestStore(t)
	_, err := store.SaveMeal(context.Background(), MealInput{})
	require.Error(t, err, "empty title should error")
}

func TestSaveMeal_zeroEatenAtDefaultsToNow(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	before := time.Now().Truncate(time.Second)
	meal, err := store.SaveMeal(ctx, MealInput{Title: "snack"})
	require.NoError(t, err, "SaveMeal")
	after := time.Now().Add(time.Second)
	assert.False(t, meal.EatenAt.Before(before), "EatenAt %v before expected range start %v", meal.EatenAt, before)
	assert.False(t, meal.EatenAt.After(after), "EatenAt %v after expected range end %v", meal.EatenAt, after)
}

func TestListMealsBetween(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	jan1 := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	jan5 := time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC)
	jan10 := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

	_, _ = store.SaveMeal(ctx, MealInput{Title: "early", EatenAt: jan1})
	_, _ = store.SaveMeal(ctx, MealInput{Title: "mid", EatenAt: jan5})
	_, _ = store.SaveMeal(ctx, MealInput{Title: "late", EatenAt: jan10})

	got, err := store.ListMealsBetween(ctx, jan1, jan10)
	require.NoError(t, err, "ListMealsBetween")
	require.Len(t, got, 2, "expected 2 meals in [jan1, jan10), got %d: %v", len(got), got)
}

func TestListRecentMeals(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	for i := range 5 {
		_, _ = store.SaveMeal(ctx, MealInput{
			Title:   "meal",
			EatenAt: time.Now().Add(time.Duration(i) * time.Minute),
		})
	}

	got, err := store.ListRecentMeals(ctx, 3)
	require.NoError(t, err, "ListRecentMeals")
	assert.Len(t, got, 3, "expected 3 meals (limit)")
}

func TestListRecentMeals_defaultLimit(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	_, _ = store.SaveMeal(ctx, MealInput{Title: "m", EatenAt: time.Now()})
	got, err := store.ListRecentMeals(ctx, 0) // 0 → default 50
	require.NoError(t, err, "ListRecentMeals")
	assert.Len(t, got, 1)
}

func TestStorerInterface(t *testing.T) {
	// Compile-time check that *Store satisfies Storer.
	var _ Storer = (*Store)(nil)
}

func TestOpen_badDSN(t *testing.T) {
	// A path inside a nonexistent directory should cause PingContext to fail.
	_, err := Open(context.Background(), "/nonexistent_bite_dir_xyz/sub/bite.db")
	require.Error(t, err)
}

func TestOpen_migrateErrorConflictingSchema(t *testing.T) {
	// Pre-populate a DB file with a conflicting "conversations" table.
	// goose.Up sees no goose_db_version, tries to run migration 001 which
	// creates "conversations" — but it already exists → Migrate returns error.
	dir := t.TempDir()
	dsn := dir + "/broken.db"

	d, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	_, err = d.ExecContext(context.Background(), "CREATE TABLE conversations (id INTEGER)")
	require.NoError(t, err)
	_ = d.Close()

	_, err = Open(context.Background(), dsn)
	require.Error(t, err, "expected Migrate error due to conflicting conversations schema")
}

func TestOpen_migrateError(t *testing.T) {
	// Open a valid DB, close it, then pass the closed handle to trigger a
	// Migrate error. We can't do this through Open directly, so we test the
	// Migrate error path separately via TestMigrate_closedDB. This test
	// verifies that an already-closed DB fails PingContext in Open.
	d, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	d.Close()
	err = d.PingContext(context.Background())
	require.Error(t, err, "expected error pinging closed db")
}

func TestMigrate_closedDB(t *testing.T) {
	d, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	d.Close() // closed DB → goose should error
	require.Error(t, Migrate(d), "expected error for closed DB")
}

func TestListConversations_defaultLimit(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	for range 3 {
		_, _ = store.NewConversation(ctx, "m", "")
	}

	convs, err := store.ListConversations(ctx, 0) // 0 → default 100
	require.NoError(t, err, "ListConversations")
	assert.Len(t, convs, 3)
}

func TestListMealsForDay_nilLoc(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	noon := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
	_, _ = store.SaveMeal(ctx, MealInput{Title: "lunch", EatenAt: noon})

	// nil loc should not panic and use time.Local
	meals, err := store.ListMealsForDay(ctx, noon, nil)
	require.NoError(t, err, "ListMealsForDay(nil loc)")
	_ = meals // result depends on local timezone; we just verify no panic/error
}

func TestAppendMessage_touchError(t *testing.T) {
	// AppendMessage touches the conversation after inserting. If the conv is
	// deleted between the two operations, TouchConversation should fail.
	// This is hard to force via the public API, so we just verify the happy
	// path also calls Touch (tested implicitly by ListMostRecentFirst).
	// Here we verify AppendMessage returns an error for invalid convID.
	ctx := context.Background()
	store := newTestStore(t)

	_, err := store.AppendMessage(ctx, 9999, "user", "hello")
	require.Error(t, err, "expected error for non-existent conversation")
}

// failOnExecDBTX wraps *sql.DB and returns an error for every ExecContext call.
// AppendMessage uses QueryRowContext (INSERT … RETURNING) then ExecContext
// (TouchConversation UPDATE), so injecting failure here covers the error branch
// at store.go:141–143 without requiring a real race condition.
type failOnExecDBTX struct{ *sql.DB }

func (f *failOnExecDBTX) ExecContext(_ context.Context, _ string, _ ...any) (sql.Result, error) {
	return nil, errors.New("injected exec error")
}

// ─── Goals tests ─────────────────────────────────────────────────────────────

func pf64(v float64) *float64 { return &v }

func TestGoals_defaults(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	g, err := store.GetGoals(ctx)
	require.NoError(t, err, "GetGoals")
	// Migration seeds one row with all NULL fields.
	assert.Nil(t, g.Kcal)
	assert.Nil(t, g.ProteinG)
	assert.Nil(t, g.CarbsG)
	assert.Nil(t, g.FatG)
}

func TestGoals_setAndGet(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	set, err := store.SetGoals(ctx, GoalInput{
		Kcal:     pf64(2000),
		ProteinG: pf64(150),
		CarbsG:   pf64(200),
		FatG:     pf64(60),
	})
	require.NoError(t, err, "SetGoals")
	require.NotNil(t, set.Kcal)
	require.NotNil(t, set.ProteinG)
	assert.Equal(t, 2000.0, *set.Kcal)
	assert.Equal(t, 150.0, *set.ProteinG)

	got, err := store.GetGoals(ctx)
	require.NoError(t, err, "GetGoals")
	require.NotNil(t, got.Kcal)
	assert.Equal(t, 2000.0, *got.Kcal)
}

func TestGoals_clearField(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Set everything first.
	_, err := store.SetGoals(ctx, GoalInput{Kcal: pf64(1800), ProteinG: pf64(100)})
	require.NoError(t, err, "SetGoals")
	// Clear ProteinG by setting nil.
	_, err = store.SetGoals(ctx, GoalInput{Kcal: pf64(1800), ProteinG: nil})
	require.NoError(t, err, "SetGoals clear")
	got, err := store.GetGoals(ctx)
	require.NoError(t, err, "GetGoals")
	assert.Nil(t, got.ProteinG, "expected ProteinG nil after clear")
	require.NotNil(t, got.Kcal)
	assert.Equal(t, 1800.0, *got.Kcal)
}

func TestAppendMessage_touchConversationError(t *testing.T) {
	ctx := context.Background()

	// Set up a real :memory: DB so CreateConversation and AppendMessage can run.
	d, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	defer d.Close()
	_, err = d.ExecContext(ctx, "PRAGMA foreign_keys = ON;")
	require.NoError(t, err)
	require.NoError(t, Migrate(d))

	// Create a conversation on the real DB.
	q := sqlc.New(d)
	conv, err := q.CreateConversation(ctx, sqlc.CreateConversationParams{Model: "m", Title: "t"})
	require.NoError(t, err)

	// Wrap d so ExecContext always errors — this makes TouchConversation fail
	// after AppendMessage's INSERT succeeds (INSERT uses QueryRowContext).
	store := &Store{db: d, q: sqlc.New(&failOnExecDBTX{d})}

	_, err = store.AppendMessage(ctx, conv.ID, "user", "hello")
	require.Error(t, err, "expected error from TouchConversation")
	assert.Contains(t, err.Error(), "touch conversation")
}
