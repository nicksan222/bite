package routes

import (
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/nicksan222/bite/internal/ai"
)

const (
	// chatSessionCookie names the cookie that maps a browser to a
	// server-side chat session. We store the conversation history
	// server-side so the htmx-driven chat.html doesn't have to
	// round-trip turns through hidden inputs.
	chatSessionCookie = "bite_chat"

	// chatSessionTTL is how long an idle session stays alive. Generous
	// because the tab stays open between turns and a thinking user is
	// normal.
	chatSessionTTL = 6 * time.Hour

	// chatSessionCookieMaxAge mirrors the server-side TTL so the cookie
	// outlives a browser restart within the window. Pre-computed
	// because cookie creation runs on every new session.
	chatSessionCookieMaxAge = int(chatSessionTTL / time.Second)
)

// chatSessionStore holds every active session in memory. Restart loses
// history, which is acceptable: the dashboard is local-only and a long
// conversation wouldn't survive an editor save anyway with `make web-dev`.
var chatSessionStore = &sessionStore{sessions: map[string]*chatSession{}}

// chatSession is one browser's conversation: the chronological list of
// turns plus a last-touch timestamp the prune sweep uses.
type chatSession struct {
	history []ai.Message
	last    time.Time
}

// sessionStore is the per-process map of chat sessions. A coarse mutex
// is fine because the dashboard is single-user and operations are
// O(small): bumping last on a known ID, snapshotting history, or
// iterating a small map to prune.
type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]*chatSession
}

// ensure returns the session ID bound to c's cookie (creating one and
// setting the cookie on first call) along with a snapshot of the
// session's history. Returning a copy — not the live *chatSession —
// keeps callers off any shared backing array, so a concurrent
// appendTurn cannot race a handler reading prior turns.
func (s *sessionStore) ensure(c fiber.Ctx) (string, []ai.Message) {
	id := c.Cookies(chatSessionCookie)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if id != "" {
		if sess, ok := s.sessions[id]; ok {
			sess.last = time.Now()
			return id, append([]ai.Message{}, sess.history...)
		}
	}
	id = newRandomID()
	s.sessions[id] = &chatSession{last: time.Now()}
	c.Cookie(&fiber.Cookie{
		Name:     chatSessionCookie,
		Value:    id,
		Path:     "/",
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
		// Mirror the request scheme: an HTTPS deployment gets a
		// Secure cookie, local HTTP dev still works.
		Secure: c.Secure(),
		// Match the server-side TTL so the cookie outlives a browser
		// restart — closing the tab and reopening within the window
		// keeps the user on the same conversation.
		MaxAge: chatSessionCookieMaxAge,
	})
	return id, nil
}

// appendTurn appends one user/assistant exchange to the session's
// history. Called by the SSE endpoint once a stream completes so the
// next turn carries proper context.
func (s *sessionStore) appendTurn(id, userMsg, assistantMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return
	}
	sess.history = append(sess.history,
		ai.Message{Role: ai.RoleUser, Content: userMsg},
		ai.Message{Role: ai.RoleAssistant, Content: assistantMsg},
	)
	sess.last = time.Now()
}

// pruneLocked drops every session whose last-touch is older than
// chatSessionTTL. The "Locked" suffix marks the contract: callers must
// hold s.mu. Called from ensure() so eviction happens on the request
// path — no background goroutine, no leaks if traffic stops.
func (s *sessionStore) pruneLocked() {
	cutoff := time.Now().Add(-chatSessionTTL)
	for id, sess := range s.sessions {
		if sess.last.Before(cutoff) {
			delete(s.sessions, id)
		}
	}
}
