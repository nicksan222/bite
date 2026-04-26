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
)

// chatSessionStore holds every active session in memory. Restart loses
// history, which is acceptable: the dashboard is local-only and a long
// conversation wouldn't survive an editor save anyway with `make web-dev`.
var chatSessionStore = &sessionStore{sessions: map[string]*chatSession{}}

type chatSession struct {
	history []ai.Message
	last    time.Time
}

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

func (s *sessionStore) pruneLocked() {
	cutoff := time.Now().Add(-chatSessionTTL)
	for id, sess := range s.sessions {
		if sess.last.Before(cutoff) {
			delete(s.sessions, id)
		}
	}
}
