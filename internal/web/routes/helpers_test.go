package routes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestMergeArgs covers the precedence rule (form wins) and the empty-
// string drop. HTML forms send "" for unfilled fields; tools must see
// them as absent so set_goals' Has-vs-zero distinction survives.
func TestMergeArgs(t *testing.T) {
	got := mergeArgs(
		map[string]string{"a": "from-query", "b": "", "c": "kept"},
		map[string]string{"a": "from-form", "d": "", "e": "added"},
	)
	require.Equal(t, "from-form", got["a"], "form must win over query")
	require.NotContains(t, got, "b", "empty query value must be dropped")
	require.NotContains(t, got, "d", "empty form value must be dropped")
	require.Equal(t, "kept", got["c"])
	require.Equal(t, "added", got["e"])
}

// TestNotFoundError_message pins that NotFoundError carries the
// requested name in its message — handlers branch on errors.As and rely
// on the message for the user-facing 404 body.
func TestNotFoundError_message(t *testing.T) {
	err := NotFoundError{Name: "missing_tool"}
	require.Contains(t, err.Error(), "missing_tool")
}
