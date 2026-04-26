package tools

import (
	"io"
	"time"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
)

// Deps is the wiring passed once at startup. Every tool's Run closure
// receives it and pulls only what it needs; tools never reach for globals.
type Deps struct {
	// Store is the persistence façade. Required for any tool that reads or
	// writes meals, conversations, or goals.
	Store db.Storer
	// AI is the streaming model client. Required for tools that send a
	// prompt (analyze_meal, log_meal_from_media, ask).
	AI ai.Streamer
	// Model is the configured Claude model name (e.g. "claude-sonnet-4-6").
	// Stored on every conversation row by PrepareSession so resumes know
	// which model produced each turn. Empty in tests that bypass cfg.
	Model string
	// Now is an injectable clock. Tests pass a fixed time; production leaves
	// it nil so NowOrDefault() returns time.Now.
	Now func() time.Time
	// Loc is the user's timezone for date arithmetic. Defaults to time.Local
	// when nil.
	Loc *time.Location
	// OpenAIAPIKey is forwarded to media.WhisperTranscriber by tools that
	// preprocess audio. Empty means audio is unsupported in that invocation.
	OpenAIAPIKey string
	// StreamWriter, when non-nil, receives progressive output from tools
	// that stream (e.g. ask). The cobra adapter wires this to stdout so
	// `bite ask` feels live; AI/slash adapters leave it nil so the model
	// gets the buffered final text via Result.Text.
	StreamWriter io.Writer
}

// NowOrDefault returns d.Now() if set, otherwise time.Now.
func (d Deps) NowOrDefault() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// LocOrDefault returns d.Loc if set, otherwise time.Local.
func (d Deps) LocOrDefault() *time.Location {
	if d.Loc != nil {
		return d.Loc
	}
	return time.Local
}

// RequireAI lets a tool that strictly needs the model (e.g. chat) fail fast
// before doing any user-facing work. If d.AI implements EnsureUsable it is
// called; otherwise this is a no-op. Test stubs don't implement EnsureUsable
// so unit tests pass through cleanly without env juggling.
func (d Deps) RequireAI() error {
	if d.AI == nil {
		return nil
	}
	if e, ok := d.AI.(aiEnsurer); ok {
		return e.EnsureUsable()
	}
	return nil
}
