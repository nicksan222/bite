package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// nil AI fails so AI-dependent tools (chat, ask, analyze_meal) all share
	// one gate instead of duplicating the nil-check at every Run site.
	err := Deps{}.RequireAI()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "AI client not configured")
}

func TestDeps_RequireAI_streamerWithoutEnsureUsable(t *testing.T) {
	// Test stubs (stubAI, eventStreamer, errStreamer) implement ai.Streamer
	// but not aiEnsurer. RequireAI must accept them without complaining so
	// unit tests don't need env juggling.
	assert.NoError(t, Deps{AI: stubAI{}}.RequireAI())
}
