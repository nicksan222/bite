package routes

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/nicksan222/bite/internal/ai"
)

// chatSessionCookie names the cookie that maps a browser to a server-side
// chat session. We store the conversation history server-side so the
// htmx-driven chat.html doesn't have to round-trip turns through hidden
// inputs.
const chatSessionCookie = "bite_chat"

// chatSessionTTL is how long an idle session stays alive. Generous
// because the tab stays open between turns and a thinking user is normal.
const chatSessionTTL = 6 * time.Hour

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

// getOrCreate returns the session bound to c's cookie, creating one (and
// setting the cookie) on first call.
func (s *sessionStore) getOrCreate(c fiber.Ctx) (string, *chatSession) {
	id := c.Cookies(chatSessionCookie)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	if id != "" {
		if sess, ok := s.sessions[id]; ok {
			sess.last = time.Now()
			return id, sess
		}
	}
	id = newRandomID()
	sess := &chatSession{last: time.Now()}
	s.sessions[id] = sess
	c.Cookie(&fiber.Cookie{
		Name:     chatSessionCookie,
		Value:    id,
		Path:     "/",
		HTTPOnly: true,
		SameSite: fiber.CookieSameSiteLaxMode,
	})
	return id, sess
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

// newRandomID returns a 32-char hex token. Used for both the session
// cookie and per-turn handoff IDs — the only requirement is global
// uniqueness within the process, not cryptographic strength.
func newRandomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
