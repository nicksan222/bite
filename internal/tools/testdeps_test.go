package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
)

// stubAI is a tiny ai.Streamer that emits a fixed response in one Delta and
// one Done event. Shared across analyze_meal, ask, and log_meal_from_media
// tests so each file doesn't redefine its own AI mock.
type stubAI struct{ resp string }

func (s stubAI) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	ch := make(chan ai.StreamEvent, 2)
	ch <- ai.StreamEvent{Delta: s.resp}
	ch <- ai.StreamEvent{Done: true, Final: s.resp}
	close(ch)
	return ch, nil
}

// errStreamer surfaces an error from Stream() before any events fire.
type errStreamer struct{ err error }

func (s errStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	return nil, s.err
}

// eventStreamer emits the supplied events in order, then closes. Lets tests
// drive specific stream-event sequences (mid-stream errors, Done-only, etc.)
// deterministically.
type eventStreamer struct{ events []ai.StreamEvent }

func (s eventStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	ch := make(chan ai.StreamEvent, len(s.events))
	for _, ev := range s.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

// freshDeps spins up a fresh in-memory SQLite store, fixed clock, and UTC loc.
// The store is closed via t.Cleanup. Model is set to a sentinel string so
// any tool that records it on a conversation/meal has something stable to
// assert against without each test setting it manually.
func freshDeps(t *testing.T) Deps {
	t.Helper()
	store, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	fixed := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	return Deps{
		Store: store,
		Model: "test-model",
		Now:   func() time.Time { return fixed },
		Loc:   time.UTC,
	}
}

// partialFailStore embeds a real Storer and lets tests inject errors for
// individual methods after a successful operation. Used to exercise the
// "second store call fails after the first succeeds" branches that closing
// the whole store can't reach (because Close makes everything fail).
type partialFailStore struct {
	db.Storer
	listMessagesErr    error
	deleteMealErr      error
	setGoalsErr        error
	newConversationErr error
}

func (p *partialFailStore) NewConversation(ctx context.Context, model, title string) (db.Conversation, error) {
	if p.newConversationErr != nil {
		return db.Conversation{}, p.newConversationErr
	}
	return p.Storer.NewConversation(ctx, model, title)
}

func (p *partialFailStore) ListMessages(ctx context.Context, convID int64) ([]db.Message, error) {
	if p.listMessagesErr != nil {
		return nil, p.listMessagesErr
	}
	return p.Storer.ListMessages(ctx, convID)
}

func (p *partialFailStore) DeleteMeal(ctx context.Context, id int64) error {
	if p.deleteMealErr != nil {
		return p.deleteMealErr
	}
	return p.Storer.DeleteMeal(ctx, id)
}

func (p *partialFailStore) SetGoals(ctx context.Context, in db.GoalInput) (db.Goal, error) {
	if p.setGoalsErr != nil {
		return db.Goal{}, p.setGoalsErr
	}
	return p.Storer.SetGoals(ctx, in)
}
