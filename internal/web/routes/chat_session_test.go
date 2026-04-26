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
