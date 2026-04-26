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
	// sessionID, history, userMsg are the handoff payload.
	sessionID string
	history   []ai.Message
	userMsg   string

	// expires is the stash-management metadata stamped by stash().
	expires time.Time
}

// turnStore is the per-process map of pending turns. Each entry is read
// at most once; the GET handler deletes on pop. A coarse mutex is fine
// because turns are short-lived and the dashboard is single-user.
var turnStore = &turnStash{pending: map[string]pendingTurn{}}

// turnStash is the per-process map of pending POST→SSE handoffs. The
// mutex covers the map and the per-entry expires field. Same coarse
// locking story as sessionStore — turns are short-lived, ops are
// O(small), and contention is bounded by the single browser tab.
type turnStash struct {
	mu      sync.Mutex
	pending map[string]pendingTurn
}

// stash records a pending turn under a fresh random ID and returns
// that ID. Each call also prunes expired entries on the request path
// so abandoned turns can't accumulate without a background sweeper.
func (s *turnStash) stash(t pendingTurn) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	id := newRandomID()
	t.expires = time.Now().Add(turnTTL)
	s.pending[id] = t
	return id
}

// pop returns the pending turn for id and removes the entry — even
// when the entry has expired. Single-shot semantics: a refresh of the
// SSE GET in the browser must not replay the conversation, so reading
// always deletes.
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

// pruneLocked drops every turn whose TTL has elapsed. The "Locked"
// suffix marks the contract: callers must hold s.mu. Called from
// stash() so eviction happens on the request path — no background
// goroutine, no leaks if traffic stops.
func (s *turnStash) pruneLocked() {
	now := time.Now()
	for id, t := range s.pending {
		if now.After(t.expires) {
			delete(s.pending, id)
		}
	}
}
