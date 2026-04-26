package routes

import (
	"html/template"
	"sync"
	"time"

	"github.com/nicksan222/bite/internal/ai"
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
var turnStore = newTurnStore()

type turnStoreT struct {
	mu      sync.Mutex
	pending map[string]pendingTurn
}

func newTurnStore() *turnStoreT {
	return &turnStoreT{pending: map[string]pendingTurn{}}
}

func (s *turnStoreT) stash(t pendingTurn) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	id := newRandomID()
	t.expires = time.Now().Add(turnTTL)
	s.pending[id] = t
	return id
}

func (s *turnStoreT) pop(id string) (pendingTurn, bool) {
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

func (s *turnStoreT) pruneLocked() {
	now := time.Now()
	for id, t := range s.pending {
		if now.After(t.expires) {
			delete(s.pending, id)
		}
	}
}

// chatTurnTmpl is the HTML fragment hx-swapped into the transcript when
// the user submits a turn. The user bubble is fully formed; the
// assistant bubble has sse-connect bound, and htmx-ext-sse appends each
// "delta" event into .chat-bubble. The "done" event closes the SSE
// connection. The session cookie carries history forward, so neither
// bubble needs hidden inputs.
var chatTurnTmpl = template.Must(template.New("chat-turn").Parse(
	`<div class="chat chat-end" data-role="user">
	<div class="chat-bubble chat-bubble-primary">{{.UserText}}</div>
</div>
<div class="chat chat-start" data-role="assistant"
     hx-ext="sse"
     sse-connect="/api/chat/stream/{{.TurnID}}"
     sse-close="done">
	<div class="chat-bubble" sse-swap="delta" hx-swap="beforeend"></div>
</div>
`))
