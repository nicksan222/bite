package routes

import (
	"sync"
	"time"

	"github.com/nicksan222/bite/internal/ai"
)

// chatStreamPath is the chat-stream route relative to the /api group,
// and chatStreamPathPrefix is its full URL prefix. Splitting the two
// lets register.go mount under api.Get(chatStreamPath+...) while the
// template (which the browser dereferences absolutely) and the tests
// reach for chatStreamPathPrefix.
const (
	chatStreamPath       = "/chat/stream/"
	chatStreamPathPrefix = "/api" + chatStreamPath
)

// turnTTL bounds how long a stashed turn can sit waiting for the SSE
// stream endpoint to pick it up. The page POSTs the form and the browser
// immediately follows with the SSE GET, so this only protects against
// orphaned turns when the user navigates away mid-flight.
const turnTTL = 30 * time.Second

// pendingTurn is one POST → SSE handoff. The POST stores everything the
// stream handler needs (history + the user's new turn + which session
// to update on completion); the GET pops it.
type pendingTurn struct {
	sessionID string
	history   []ai.Message
	userMsg   string
	expires   time.Time
}

// turnStore is the per-process map of pending turns. Each entry is read
// at most once; the GET handler deletes on pop. A coarse mutex is fine
// because turns are short-lived and the dashboard is single-user.
var turnStore = &turnStash{pending: map[string]pendingTurn{}}

type turnStash struct {
	mu      sync.Mutex
	pending map[string]pendingTurn
}

func (s *turnStash) stash(t pendingTurn) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	id := newRandomID()
	t.expires = time.Now().Add(turnTTL)
	s.pending[id] = t
	return id
}

func (s *turnStash) pop(id string) (pendingTurn, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.pending[id]
	if !ok {
		return pendingTurn{}, false
	}
	delete(s.pending, id)
	if time.Now().After(t.expires) {
		return pendingTurn{}, false
	}
	return t, true
}

func (s *turnStash) pruneLocked() {
	now := time.Now()
	for id, t := range s.pending {
		if now.After(t.expires) {
			delete(s.pending, id)
		}
	}
}
