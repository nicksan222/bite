package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/nicksan222/bite/internal/db"
)

// mockStore is an in-memory implementation of db.Storer for CLI tests.
type mockStore struct {
	convs         []db.Conversation
	msgs          []db.Message
	meals         []db.Meal
	goals         db.Goal
	nextID        int64
	closeErr      error
	listConvsErr  error // injected error for ListConversations
	listMsgsErr   error // injected error for ListMessages
	appendMsgErr  error // injected error for AppendMessage
	newConvErr    error // injected error for NewConversation
	listMealsErr  error // injected error for ListMealsForDay / ListRecentMeals
	getMealErr    error // injected error for GetMeal
	deleteMealErr error // injected error for DeleteMeal
	goalsErr      error // injected error for GetGoals
	setGoalsErr   error // injected error for SetGoals only
	streakVal     int   // returned by Streak
	streakErr     error // injected error for Streak
}

func newMockStore() *mockStore { return &mockStore{nextID: 1} }

func (m *mockStore) nextID64() int64 {
	id := m.nextID
	m.nextID++
	return id
}

func (m *mockStore) NewConversation(_ context.Context, model, title string) (db.Conversation, error) {
	if m.newConvErr != nil {
		return db.Conversation{}, m.newConvErr
	}
	c := db.Conversation{
		ID:        m.nextID64(),
		Model:     model,
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.convs = append(m.convs, c)
	return c, nil
}

func (m *mockStore) GetConversation(_ context.Context, id int64) (db.Conversation, error) {
	for _, c := range m.convs {
		if c.ID == id {
			return c, nil
		}
	}
	return db.Conversation{}, fmt.Errorf("conversation %d not found", id)
}

func (m *mockStore) ListConversations(_ context.Context, limit int64) ([]db.Conversation, error) {
	if m.listConvsErr != nil {
		return nil, m.listConvsErr
	}
	out := m.convs
	if int64(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *mockStore) RenameConversation(_ context.Context, id int64, title string) error {
	for i, c := range m.convs {
		if c.ID == id {
			m.convs[i].Title = title
			return nil
		}
	}
	return fmt.Errorf("conversation %d not found", id)
}

func (m *mockStore) DeleteConversation(_ context.Context, id int64) error {
	for i, c := range m.convs {
		if c.ID == id {
			m.convs = append(m.convs[:i], m.convs[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("conversation %d not found", id)
}

func (m *mockStore) TouchConversation(_ context.Context, id int64) error {
	for i, c := range m.convs {
		if c.ID == id {
			m.convs[i].UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("conversation %d not found", id)
}

func (m *mockStore) AppendMessage(_ context.Context, convID int64, role, content string) (db.Message, error) {
	if m.appendMsgErr != nil {
		return db.Message{}, m.appendMsgErr
	}
	msg := db.Message{
		ID:             m.nextID64(),
		ConversationID: convID,
		Role:           role,
		Content:        content,
		CreatedAt:      time.Now(),
	}
	m.msgs = append(m.msgs, msg)
	return msg, nil
}

func (m *mockStore) ListMessages(_ context.Context, convID int64) ([]db.Message, error) {
	if m.listMsgsErr != nil {
		return nil, m.listMsgsErr
	}
	var out []db.Message
	for _, msg := range m.msgs {
		if msg.ConversationID == convID {
			out = append(out, msg)
		}
	}
	return out, nil
}

func (m *mockStore) SaveMeal(_ context.Context, in db.MealInput) (db.Meal, error) {
	if in.Title == "" {
		return db.Meal{}, fmt.Errorf("title is required")
	}
	eatAt := in.EatenAt
	if eatAt.IsZero() {
		eatAt = time.Now()
	}
	meal := db.Meal{
		ID:          m.nextID64(),
		Title:       in.Title,
		Description: in.Description,
		Items:       "[]",
		Kcal:        in.Kcal,
		ProteinG:    in.ProteinG,
		CarbsG:      in.CarbsG,
		FatG:        in.FatG,
		EatenAt:     eatAt,
		CreatedAt:   time.Now(),
	}
	m.meals = append(m.meals, meal)
	return meal, nil
}

func (m *mockStore) GetMeal(_ context.Context, id int64) (db.Meal, error) {
	if m.getMealErr != nil {
		return db.Meal{}, m.getMealErr
	}
	for _, meal := range m.meals {
		if meal.ID == id {
			return meal, nil
		}
	}
	return db.Meal{}, fmt.Errorf("meal %d not found", id)
}

func (m *mockStore) DeleteMeal(_ context.Context, id int64) error {
	if m.deleteMealErr != nil {
		return m.deleteMealErr
	}
	for i, meal := range m.meals {
		if meal.ID == id {
			m.meals = append(m.meals[:i], m.meals[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("meal %d not found", id)
}

func (m *mockStore) ListMealsForDay(_ context.Context, now time.Time, loc *time.Location) ([]db.Meal, error) {
	if m.listMealsErr != nil {
		return nil, m.listMealsErr
	}
	if loc == nil {
		loc = time.Local
	}
	y, mo, d := now.In(loc).Date()
	var out []db.Meal
	for _, meal := range m.meals {
		my, mmo, md := meal.EatenAt.In(loc).Date()
		if my == y && mmo == mo && md == d {
			out = append(out, meal)
		}
	}
	return out, nil
}

func (m *mockStore) ListMealsBetween(_ context.Context, from, to time.Time) ([]db.Meal, error) {
	if m.listMealsErr != nil {
		return nil, m.listMealsErr
	}
	var out []db.Meal
	for _, meal := range m.meals {
		if !meal.EatenAt.Before(from) && meal.EatenAt.Before(to) {
			out = append(out, meal)
		}
	}
	return out, nil
}

func (m *mockStore) ListRecentMeals(_ context.Context, limit int64) ([]db.Meal, error) {
	if m.listMealsErr != nil {
		return nil, m.listMealsErr
	}
	if limit <= 0 {
		limit = 50
	}
	out := make([]db.Meal, len(m.meals))
	copy(out, m.meals)
	// reverse for most-recent-first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if int64(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *mockStore) Streak(_ context.Context, _ time.Time, _ *time.Location) (int, error) {
	return m.streakVal, m.streakErr
}

func (m *mockStore) GetGoals(_ context.Context) (db.Goal, error) {
	if m.goalsErr != nil {
		return db.Goal{}, m.goalsErr
	}
	return m.goals, nil
}

func (m *mockStore) SetGoals(_ context.Context, in db.GoalInput) (db.Goal, error) {
	if m.setGoalsErr != nil {
		return db.Goal{}, m.setGoalsErr
	}
	m.goals.Kcal = in.Kcal
	m.goals.ProteinG = in.ProteinG
	m.goals.CarbsG = in.CarbsG
	m.goals.FatG = in.FatG
	return m.goals, nil
}

func (m *mockStore) Close() error { return m.closeErr }

// compile-time: mockStore must satisfy db.Storer.
var _ db.Storer = (*mockStore)(nil)
