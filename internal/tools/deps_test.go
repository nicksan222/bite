package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDeps_NowOrDefault_fallback(t *testing.T) {
	// Empty Deps (no Now func) returns time.Now-derived value.
	d := Deps{}
	got := d.NowOrDefault()
	assert.WithinDuration(t, time.Now(), got, time.Second,
		"NowOrDefault with nil Now must fall back to time.Now")
}

func TestDeps_LocOrDefault_fallback(t *testing.T) {
	// Empty Deps (no Loc) returns time.Local.
	d := Deps{}
	assert.Equal(t, time.Local, d.LocOrDefault())
}

func TestDeps_RequireAI_nilAI(t *testing.T) {
	// Tools without AI needs construct Deps without setting AI; RequireAI
	// must be a no-op so they don't have to bypass the gate.
	assert.NoError(t, Deps{}.RequireAI())
}

func TestDeps_RequireAI_streamerWithoutEnsureUsable(t *testing.T) {
	// Test stubs (stubAI, eventStreamer, errStreamer) implement ai.Streamer
	// but not aiEnsurer. RequireAI must accept them without complaining so
	// unit tests don't need env juggling.
	assert.NoError(t, Deps{AI: stubAI{}}.RequireAI())
}
