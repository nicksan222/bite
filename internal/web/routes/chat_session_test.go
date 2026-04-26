package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSessionStore_appendTurn_unknownIDIsNoop pins the contract that
// appending to a missing session is silent — the SSE handler may race
// with session expiry and shouldn't blow up if the session is gone.
func TestSessionStore_appendTurn_unknownIDIsNoop(t *testing.T) {
	s := &sessionStore{sessions: map[string]*chatSession{}}
	require.NotPanics(t, func() {
		s.appendTurn("nope", "user", "asst")
	})
}

// TestSessionStore_appendTurn_refreshesLastTouched proves appendTurn
// bumps sess.last. Without this, a session that takes most of the TTL
// to assemble its first reply could be pruned right after the SSE
// stream completes — losing the asst message that was just appended.
func TestSessionStore_appendTurn_refreshesLastTouched(t *testing.T) {
	stale := time.Now().Add(-2 * chatSessionTTL)
	s := &sessionStore{sessions: map[string]*chatSession{
		"s1": {last: stale},
	}}
	s.appendTurn("s1", "u", "a")
	require.True(t, s.sessions["s1"].last.After(stale),
		"appendTurn must refresh last so the just-completed turn isn't pruned next")
}

// TestSessionStore_pruneEvictsIdle proves pruneLocked actually removes
// entries past the TTL — without this, sessions would leak forever.
func TestSessionStore_pruneEvictsIdle(t *testing.T) {
	s := &sessionStore{sessions: map[string]*chatSession{
		"alive": {last: time.Now()},
		"stale": {last: time.Now().Add(-2 * chatSessionTTL)},
	}}
	s.mu.Lock()
	s.pruneLocked()
	s.mu.Unlock()
	require.Contains(t, s.sessions, "alive")
	require.NotContains(t, s.sessions, "stale")
}
