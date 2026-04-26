package routes

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
}

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
