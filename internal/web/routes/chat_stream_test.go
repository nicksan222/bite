package routes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// TestBuildChatHistory_appendsNewUserAtTail proves the contract chat.js
// relies on: the JSON `message` field becomes the last entry, with the
// `history` entries preceding it in order. Disjoint from history (the
// new turn must not appear twice).
func TestBuildChatHistory_appendsNewUserAtTail(t *testing.T) {
	got, err := buildChatHistory(chatRequest{
		Message: "third",
		History: []chatMsgDTO{
			{Role: "user", Content: "first"},
			{Role: "assistant", Content: "second"},
		},
	})
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, ai.Message{Role: ai.RoleUser, Content: "first"}, got[0])
	require.Equal(t, ai.Message{Role: ai.RoleAssistant, Content: "second"}, got[1])
	require.Equal(t, ai.Message{Role: ai.RoleUser, Content: "third"}, got[2])
}

// TestBuildChatHistory_emptyHistory covers the cold-start case: with no
// prior turns the result is just the new user message.
func TestBuildChatHistory_emptyHistory(t *testing.T) {
	got, err := buildChatHistory(chatRequest{Message: "hi"})
	require.NoError(t, err)
	require.Equal(t, []ai.Message{{Role: ai.RoleUser, Content: "hi"}}, got)
}

// TestBuildChatHistory_invalidRole locks in the role allowlist (covers
// the "tool"/"" cases the API-level test doesn't exercise directly).
// The error message must point at the offending index so a developer
// can see *which* history entry is the problem.
func TestBuildChatHistory_invalidRole(t *testing.T) {
	for _, bad := range []string{"system", "tool", "", "User"} {
		_, err := buildChatHistory(chatRequest{
			Message: "hi",
			History: []chatMsgDTO{
				{Role: "user", Content: "first"},
				{Role: bad, Content: "second"},
			},
		})
		require.Error(t, err, "role=%q must be rejected", bad)
		require.Contains(t, err.Error(), "history[1]", "error must locate the offending index")
	}
}
