package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/nicksan222/bite/internal/db/sqlc"
)

// Conversation, Message, Meal, and Goal are re-exported from the generated layer
// so that callers depend only on package db, not db/sqlc.
type (
	Conversation = sqlc.Conversation
	Message      = sqlc.Message
	Meal         = sqlc.Meal
	Goal         = sqlc.Goal
)

// GoalInput holds the user's desired daily targets. Any nil field means
// "no target set" — the value is cleared from the DB.
type GoalInput struct {
	Kcal     *float64
	ProteinG *float64
	CarbsG   *float64
	FatG     *float64
}

// Storer is the interface bite's CLI and TUI layers depend on.
// *Store implements Storer.
type Storer interface {
	// Conversations
	NewConversation(ctx context.Context, model, title string) (Conversation, error)
	GetConversation(ctx context.Context, id int64) (Conversation, error)
	ListConversations(ctx context.Context, limit int64) ([]Conversation, error)
	RenameConversation(ctx context.Context, id int64, title string) error
	DeleteConversation(ctx context.Context, id int64) error
	TouchConversation(ctx context.Context, id int64) error

	// Messages
	AppendMessage(ctx context.Context, convID int64, role, content string) (Message, error)
	ListMessages(ctx context.Context, convID int64) ([]Message, error)

	// Meals
	SaveMeal(ctx context.Context, in MealInput) (Meal, error)
	GetMeal(ctx context.Context, id int64) (Meal, error)
	DeleteMeal(ctx context.Context, id int64) error
	ListMealsForDay(ctx context.Context, day time.Time, loc *time.Location) ([]Meal, error)
	ListMealsBetween(ctx context.Context, from, to time.Time) ([]Meal, error)
	ListRecentMeals(ctx context.Context, limit int64) ([]Meal, error)

	// Goals
	GetGoals(ctx context.Context) (Goal, error)
	SetGoals(ctx context.Context, in GoalInput) (Goal, error)

	// Streak
	Streak(ctx context.Context, now time.Time, loc *time.Location) (int, error)

	// Preferences
	GetPreference(ctx context.Context, key string) (value string, ok bool, err error)
	SetPreference(ctx context.Context, key, value string) error

	Close() error
}

// compile-time assertion
var _ Storer = (*Store)(nil)

// Store is the high-level data-access façade for bite.
// It owns the *sql.DB and exposes typed methods backed by sqlc-generated code.
//
// All persistence calls in the rest of the codebase should go through *Store.
// To add a new operation, add a method here that wraps one or more sqlc calls.
type Store struct {
	db *sql.DB
	q  *sqlc.Queries
}

// Open opens (and migrates) a SQLite database at the given DSN.
// Pass ":memory:" for an ephemeral DB.
func Open(ctx context.Context, dsn string) (*Store, error) {
	d, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := d.PingContext(ctx); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	if _, err := d.ExecContext(ctx, "PRAGMA foreign_keys = ON;"); err != nil {
		_ = d.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if err := Migrate(d); err != nil {
		_ = d.Close()
		return nil, err
	}

	return &Store{db: d, q: sqlc.New(d)}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// ─── Conversations ──────────────────────────────────────────────────────────

func (s *Store) NewConversation(ctx context.Context, model, title string) (Conversation, error) {
	return s.q.CreateConversation(ctx, sqlc.CreateConversationParams{
		Title: title,
		Model: model,
	})
}

func (s *Store) GetConversation(ctx context.Context, id int64) (Conversation, error) {
	return s.q.GetConversation(ctx, id)
}

func (s *Store) ListConversations(ctx context.Context, limit int64) ([]Conversation, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.q.ListConversations(ctx, limit)
}

func (s *Store) RenameConversation(ctx context.Context, id int64, title string) error {
	return s.q.UpdateConversationTitle(ctx, sqlc.UpdateConversationTitleParams{
		ID:    id,
		Title: title,
	})
}

func (s *Store) TouchConversation(ctx context.Context, id int64) error {
	return s.q.TouchConversation(ctx, id)
}

func (s *Store) DeleteConversation(ctx context.Context, id int64) error {
	return s.q.DeleteConversation(ctx, id)
}

// ─── Messages ───────────────────────────────────────────────────────────────

func (s *Store) AppendMessage(ctx context.Context, convID int64, role, content string) (Message, error) {
	m, err := s.q.AppendMessage(ctx, sqlc.AppendMessageParams{
		ConversationID: convID,
		Role:           role,
		Content:        content,
	})
	if err != nil {
		return Message{}, fmt.Errorf("append message: %w", err)
	}
	if err := s.q.TouchConversation(ctx, convID); err != nil {
		return m, fmt.Errorf("touch conversation: %w", err)
	}
	return m, nil
}

func (s *Store) ListMessages(ctx context.Context, convID int64) ([]Message, error) {
	return s.q.ListMessages(ctx, convID)
}

func (s *Store) CountMessages(ctx context.Context, convID int64) (int64, error) {
	return s.q.CountMessages(ctx, convID)
}

// ─── Meals ──────────────────────────────────────────────────────────────────

// MealInput is the value-shape for saving a meal. Items is stored as JSON.
type MealInput struct {
	Title       string
	Description string
	Items       []string
	Kcal        float64
	ProteinG    float64
	CarbsG      float64
	FatG        float64
	EatenAt     time.Time // zero = now
}

func (s *Store) SaveMeal(ctx context.Context, in MealInput) (Meal, error) {
	if in.Title == "" {
		return Meal{}, fmt.Errorf("save meal: title is required")
	}
	if in.EatenAt.IsZero() {
		in.EatenAt = time.Now()
	}
	itemsJSON, err := json.Marshal(in.Items)
	if err != nil {
		return Meal{}, fmt.Errorf("encode items: %w", err)
	}
	return s.q.SaveMeal(ctx, sqlc.SaveMealParams{
		Title:       in.Title,
		Description: in.Description,
		Items:       string(itemsJSON),
		Kcal:        in.Kcal,
		ProteinG:    in.ProteinG,
		CarbsG:      in.CarbsG,
		FatG:        in.FatG,
		EatenAt:     in.EatenAt,
	})
}

func (s *Store) GetMeal(ctx context.Context, id int64) (Meal, error) {
	return s.q.GetMeal(ctx, id)
}

func (s *Store) DeleteMeal(ctx context.Context, id int64) error {
	return s.q.DeleteMeal(ctx, id)
}

// ListMealsForDay returns every meal eaten on the given calendar date in the
// given timezone. Pass time.Local for the user's local day boundaries.
func (s *Store) ListMealsForDay(ctx context.Context, day time.Time, loc *time.Location) ([]Meal, error) {
	if loc == nil {
		loc = time.Local
	}
	y, m, d := day.In(loc).Date()
	start := time.Date(y, m, d, 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	return s.q.ListMealsBetween(ctx, sqlc.ListMealsBetweenParams{
		EatenAt:   start,
		EatenAt_2: end,
	})
}

func (s *Store) ListMealsBetween(ctx context.Context, from, to time.Time) ([]Meal, error) {
	return s.q.ListMealsBetween(ctx, sqlc.ListMealsBetweenParams{
		EatenAt:   from,
		EatenAt_2: to,
	})
}

func (s *Store) ListRecentMeals(ctx context.Context, limit int64) ([]Meal, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.q.ListRecentMeals(ctx, limit)
}

// ─── Goals ───────────────────────────────────────────────────────────────────

func (s *Store) GetGoals(ctx context.Context) (Goal, error) {
	return s.q.GetGoals(ctx)
}

func (s *Store) SetGoals(ctx context.Context, in GoalInput) (Goal, error) {
	return s.q.SetGoals(ctx, sqlc.SetGoalsParams{
		Kcal:     in.Kcal,
		ProteinG: in.ProteinG,
		CarbsG:   in.CarbsG,
		FatG:     in.FatG,
	})
}

// ─── Preferences ────────────────────────────────────────────────────────────

// GetPreference returns ok=false (and no error) when the key is absent
// so callers don't need errors.Is(sql.ErrNoRows, …).
func (s *Store) GetPreference(ctx context.Context, key string) (string, bool, error) {
	v, err := s.q.GetPreference(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get preference %q: %w", key, err)
	}
	return v, true, nil
}

func (s *Store) SetPreference(ctx context.Context, key, value string) error {
	if err := s.q.SetPreference(ctx, sqlc.SetPreferenceParams{Key: key, Value: value}); err != nil {
		return fmt.Errorf("set preference %q: %w", key, err)
	}
	return nil
}
