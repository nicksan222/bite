package routes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRenderChatTurn_escapesUserText is the load-bearing security
// assertion: anything the user types lands inside the user bubble via
// html/template auto-escape, so a `<script>` payload renders as text
// instead of executing.
func TestRenderChatTurn_escapesUserText(t *testing.T) {
	html, err := renderChatTurn("abcd1234", `<script>alert(1)</script>`)
	require.NoError(t, err)
	require.NotContains(t, string(html), `<script>`)
	require.Contains(t, string(html), `&lt;script&gt;alert(1)&lt;/script&gt;`)
}

// TestRenderChatTurn_structure pins the dual-bubble shape: a user
// chat-end, an assistant chat-start with sse-connect to the matching
// turn ID, plus the delta and error swap targets htmx-ext-sse drives.
func TestRenderChatTurn_structure(t *testing.T) {
	html, err := renderChatTurn("turn123", "hi")
	require.NoError(t, err)
	got := string(html)
	require.Contains(t, got, `chat chat-end`)
	require.Contains(t, got, `chat chat-start`)
	require.Contains(t, got, `sse-connect="`+chatStreamPathPrefix+`turn123"`)
	require.Contains(t, got, `sse-swap="delta"`)
	require.Contains(t, got, `sse-swap="error"`)
}
