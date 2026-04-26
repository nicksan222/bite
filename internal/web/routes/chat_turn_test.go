package routes

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestTurnStash_popExpiredReturnsNotFound pins that an expired entry is
// reported as missing — the SSE handler treats {ok:false} as a 404, so
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

// TestRenderChatTurn_escapesUserText is the load-bearing security
// assertion: anything the user types lands inside the user bubble via
// html/template auto-escape, so a `<script>` payload renders as text
// instead of executing.
func TestRenderChatTurn_escapesUserText(t *testing.T) {
	html, err := renderChatTurn("abcd1234", `<script>alert(1)</script>`)
	require.NoError(t, err)
	require.NotContains(t, string(html), `<script>`)
	require.Contains(t, string(html), `&lt;script&gt;alert(1)&lt;/script&gt;`)
}

// TestRenderChatTurn_structure pins the dual-bubble shape: a user
// chat-end, an assistant chat-start with sse-connect to the matching
// turn ID, plus the delta and error swap targets htmx-ext-sse drives.
func TestRenderChatTurn_structure(t *testing.T) {
	html, err := renderChatTurn("turn123", "hi")
	require.NoError(t, err)
	got := string(html)
	require.Contains(t, got, `chat chat-end`)
	require.Contains(t, got, `chat chat-start`)
	require.Contains(t, got, `sse-connect="/api/chat/stream/turn123"`)
	require.Contains(t, got, `sse-swap="delta"`)
	require.Contains(t, got, `sse-swap="error"`)
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
