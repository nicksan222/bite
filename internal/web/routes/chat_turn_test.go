package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTurnStash_popExpiredReturnsNotFound pins that an expired entry is
// reported as missing. The SSE handler converts pop's (zero, false)
// into an `event: error\ndata: turn expired or not found` payload, so
// an expired turn must not pretend to be valid.
func TestTurnStash_popExpiredReturnsNotFound(t *testing.T) {
	s := &turnStash{pending: map[string]pendingTurn{}}
	id := s.stash(pendingTurn{sessionID: "x"})
	// Force expiry without waiting.
	s.mu.Lock()
	t2 := s.pending[id]
	t2.expires = time.Now().Add(-time.Second)
	s.pending[id] = t2
	s.mu.Unlock()

	_, ok := s.pop(id)
	require.False(t, ok, "expired turn must report missing")
	require.NotContains(t, s.pending, id, "expired pop should still delete the entry so it can't be retried")
}

// TestTurnStash_stash_prunesExpiredEntries pins that prune runs as part
// of every stash call. Without prune in the hot path, abandoned turns
// would only be evicted by direct pruneLocked calls — leaks if traffic
// patterns ever change.
func TestTurnStash_stash_prunesExpiredEntries(t *testing.T) {
	s := &turnStash{pending: map[string]pendingTurn{
		"expired": {expires: time.Now().Add(-time.Second)},
	}}
	s.stash(pendingTurn{sessionID: "x"})
	require.NotContains(t, s.pending, "expired",
		"stash must call pruneLocked so abandoned turns don't leak")
}

// TestTurnStash_pruneEvictsExpired covers the explicit pruneLocked path.
// stash() also prunes incidentally, but we want the helper itself
// covered so a future refactor doesn't accidentally delete it.
func TestTurnStash_pruneEvictsExpired(t *testing.T) {
	s := &turnStash{pending: map[string]pendingTurn{
		"alive":   {expires: time.Now().Add(time.Minute)},
		"expired": {expires: time.Now().Add(-time.Second)},
	}}
	s.mu.Lock()
	s.pruneLocked()
	s.mu.Unlock()
	require.Contains(t, s.pending, "alive")
	require.NotContains(t, s.pending, "expired")
}
