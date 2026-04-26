package routes

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// TestSessionStore_cookieSecuritySettings pins the chat-session cookie's
// security flags. HttpOnly so JS can't exfiltrate the session ID,
// SameSite=Lax so cross-site POSTs don't carry it (CSRF guard while
// still allowing top-level navigation), and Path=/ because every chat
// route shares the cookie. The Secure flag is exercised separately
// because it depends on the request scheme.
func TestSessionStore_cookieSecuritySettings(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{}})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	cookies := resp.Cookies()
	require.Len(t, cookies, 1, "first POST must set exactly one cookie")
	c := cookies[0]
	require.Equal(t, chatSessionCookie, c.Name)
	require.True(t, c.HttpOnly, "session cookie must be HttpOnly")
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.Equal(t, "/", c.Path)
	require.False(t, c.Secure, "Secure must be false on HTTP test requests")
	require.Equal(t, chatSessionCookieMaxAge, c.MaxAge,
		"MaxAge must match server TTL so closing the browser doesn't drop history that's still alive on the server")
	require.Equal(t, 6*60*60, c.MaxAge, "TTL constant must be 6h in seconds")
}

// TestSessionStore_appendTurn_unknownIDIsNoop pins the contract that
// appending to a missing session is silent and inert — the SSE handler
// may race with session expiry and shouldn't blow up if the session is
// gone, nor should it resurrect the session by writing under the
// missing ID.
func TestSessionStore_appendTurn_unknownIDIsNoop(t *testing.T) {
	s := &sessionStore{sessions: map[string]*chatSession{}}
	require.NotPanics(t, func() {
		s.appendTurn("nope", "user", "asst")
	})
	require.Empty(t, s.sessions, "appendTurn must not create a session for an unknown ID")
}

// TestSessionStore_appendTurn_refreshesLastTouched proves appendTurn
// bumps sess.last AND actually appends the user/asst pair to history.
// Without the timestamp bump, a session that takes most of the TTL to
// assemble its first reply could be pruned right after the SSE stream
// completes — losing the asst message just appended. Without the
// history append, subsequent turns wouldn't carry context.
func TestSessionStore_appendTurn_refreshesLastTouched(t *testing.T) {
	stale := time.Now().Add(-2 * chatSessionTTL)
	s := &sessionStore{sessions: map[string]*chatSession{
		"s1": {last: stale},
	}}
	s.appendTurn("s1", "u", "a")
	sess := s.sessions["s1"]
	require.True(t, sess.last.After(stale),
		"appendTurn must refresh last so the just-completed turn isn't pruned next")
	require.Equal(t, []ai.Message{
		{Role: ai.RoleUser, Content: "u"},
		{Role: ai.RoleAssistant, Content: "a"},
	}, sess.history, "appendTurn must record the full user/asst exchange in order")
}

// TestSessionStore_ensure_prunesIdleEntries pins that prune runs as
// part of every ensure call. Without prune in the request path,
// sessions would only be evicted if pruneLocked were called by some
// other code path — making leaks possible if traffic patterns change.
func TestSessionStore_ensure_prunesIdleEntries(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{}})
	// Plant a stale session in the global store and submit a request
	// through ensure. The stale entry should disappear; the request's
	// own session should appear. Cleanup resets the store so a leak
	// from this test can't pollute downstream cases.
	chatSessionStore.mu.Lock()
	chatSessionStore.sessions["stale-id"] = &chatSession{last: time.Now().Add(-2 * chatSessionTTL)}
	chatSessionStore.mu.Unlock()
	t.Cleanup(func() {
		chatSessionStore.mu.Lock()
		chatSessionStore.sessions = map[string]*chatSession{}
		chatSessionStore.mu.Unlock()
	})

	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	chatSessionStore.mu.Lock()
	defer chatSessionStore.mu.Unlock()
	require.NotContains(t, chatSessionStore.sessions, "stale-id",
		"ensure must call pruneLocked so idle sessions don't leak")
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
