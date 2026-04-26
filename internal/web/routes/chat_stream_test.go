package routes

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// stubStreamer returns the configured deltas, then either a terminal
// error or a terminating Done. Used by chat tests that need to drive the
// SSE pipeline without a real model. gotMessages and gotOptCount expose
// what Stream observed so tests can assert the handler forwarded them.
type stubStreamer struct {
	deltas   []string
	final    string
	streamer error // returned synchronously from Stream — simulates handshake failure
	tail     error // emitted as a terminal ev.Err — simulates mid-stream failure

	gotMessages []ai.Message
	gotOptCount int
}

func (s *stubStreamer) Stream(_ context.Context, msgs []ai.Message, opts ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	s.gotMessages = msgs
	s.gotOptCount = len(opts)
	if s.streamer != nil {
		return nil, s.streamer
	}
	ch := make(chan ai.StreamEvent, len(s.deltas)+1)
	for _, d := range s.deltas {
		ch <- ai.StreamEvent{Delta: d}
	}
	if s.tail != nil {
		ch <- ai.StreamEvent{Err: s.tail}
	} else {
		ch <- ai.StreamEvent{Done: true, Final: s.final}
	}
	close(ch)
	return ch, nil
}

// postForm is the form-encoded equivalent of postJSON for chat tests.
func postForm(path string, fields map[string]string) *http.Request {
	values := url.Values{}
	for k, v := range fields {
		values.Set(k, v)
	}
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestChatStart_emptyMessage(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{}})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "   "}))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestChatStart_unconfigured(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html",
		"unconfigured AI must surface as an HTML alert (the form's hx-target is the transcript)")
}

// TestChatStart_returnsBubblesWithSSEConnect proves the POST handler
// returns the dual-bubble HTML scaffold and the assistant bubble carries
// the sse-connect attribute pointing at a fresh stream URL.
func TestChatStart_returnsBubblesWithSSEConnect(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{}})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi there"}))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	require.Contains(t, got, `chat chat-end`)
	require.Contains(t, got, `chat chat-start`)
	require.Contains(t, got, `hi there`)
	require.Contains(t, got, `hx-ext="sse"`)
	require.Regexp(t, regexp.MustCompile(`sse-connect="/api/chat/stream/[0-9a-f]{32}"`), got)
}

// TestChatTurn_lifecycle drives the full POST → SSE round-trip: the
// turn ID returned from POST must accept exactly one GET, and the
// stream must include the user's message in the model's history.
func TestChatTurn_lifecycle(t *testing.T) {
	streamer := &stubStreamer{deltas: []string{"hello"}, final: "hello"}
	app := newApp(Deps{AI: streamer})

	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	turnID := extractTurnID(t, string(body))

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/"+turnID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	streamBody, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(streamBody), "event: delta")
	require.Contains(t, string(streamBody), "data: hello")
	require.Contains(t, string(streamBody), "event: done")

	require.Len(t, streamer.gotMessages, 1)
	require.Equal(t, ai.RoleUser, streamer.gotMessages[0].Role)
	require.Equal(t, "hi", streamer.gotMessages[0].Content)
}

// TestChatTurn_idIsSingleUse locks in the contract that a turn ID can
// only be popped once. A second GET must 404 — otherwise a refresh in
// the browser would replay the conversation.
func TestChatTurn_idIsSingleUse(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{deltas: []string{"x"}, final: "x"}})

	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	turnID := extractTurnID(t, string(body))

	first, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/"+turnID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, first.StatusCode)
	_, _ = io.ReadAll(first.Body)

	second, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/"+turnID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, second.StatusCode)
}

// TestChatTurn_unknownIdIs404 exercises the cold path: a GET for an ID
// that was never stashed (link forge, restart, etc).
func TestChatTurn_unknownIdIs404(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{}})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/deadbeef", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestChatStream_unconfigured covers the AI-not-configured path on the
// SSE endpoint specifically (the POST has its own check above).
func TestChatStream_unconfigured(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/whatever", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

// TestChatTurn_sessionAccumulates: the second turn's history must
// include the assistant reply from the first turn. Proves the session
// store is wired and the post-stream history append actually happens.
func TestChatTurn_sessionAccumulates(t *testing.T) {
	streamer := &stubStreamer{deltas: []string{"answer"}, final: "answer"}
	app := newApp(Deps{AI: streamer})

	// Turn 1
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "first"}))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	cookie := resp.Cookies()
	require.NotEmpty(t, cookie, "first POST must set the session cookie")
	turn1 := extractTurnID(t, string(body))

	// Drain the SSE so the asst reply is appended to the session.
	streamResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/"+turn1, nil))
	require.NoError(t, err)
	_, _ = io.ReadAll(streamResp.Body)

	// Turn 2 — must reuse the cookie.
	req := postForm("/api/chat", map[string]string{"message": "second"})
	req.AddCookie(cookie[0])
	_, err = app.Test(req)
	require.NoError(t, err)

	// Trigger streaming for turn 2 so we capture the messages it forwards.
	resp2, err := app.Test(req)
	require.NoError(t, err)
	body2, _ := io.ReadAll(resp2.Body)
	turn2 := extractTurnID(t, string(body2))
	streamResp2, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/chat/stream/"+turn2, nil))
	require.NoError(t, err)
	_, _ = io.ReadAll(streamResp2.Body)

	require.GreaterOrEqual(t, len(streamer.gotMessages), 3,
		"second turn should include prior user+asst plus the new user message")
	roles := []ai.Role{}
	for _, m := range streamer.gotMessages {
		roles = append(roles, m.Role)
	}
	require.Contains(t, roles, ai.RoleAssistant, "history must carry the prior assistant reply")
}

// TestPumpStreamEvents_clientDisconnectStopsLoop proves that once a
// write to the SSE stream fails (client closed the connection), the
// pump exits immediately rather than continuing to drain the channel.
// Otherwise a slow-disconnecting client would force the model goroutine
// to keep producing tokens we'd silently throw away.
func TestPumpStreamEvents_clientDisconnectStopsLoop(t *testing.T) {
	ch := make(chan ai.StreamEvent, 5)
	for i := 0; i < 5; i++ {
		ch <- ai.StreamEvent{Delta: "tok"}
	}
	close(ch)

	w := bufio.NewWriter(&failingWriter{failAt: 1})
	final := pumpStreamEvents(w, ch)
	require.Empty(t, final, "client-disconnect path must not return a final string")
	require.NotEmpty(t, ch, "pump must stop draining the channel once write fails")
}

// TestPumpStreamEvents_terminalErrorYieldsErrorEvent validates the
// mid-stream failure path: an ev.Err drains as `event: error` and the
// pump returns "" (so the session does NOT get a partial assistant
// message appended).
func TestPumpStreamEvents_terminalErrorYieldsErrorEvent(t *testing.T) {
	ch := make(chan ai.StreamEvent, 2)
	ch <- ai.StreamEvent{Delta: "part"}
	ch <- ai.StreamEvent{Err: errors.New("boom")}
	close(ch)

	var buf strings.Builder
	w := bufio.NewWriter(&stringWriter{b: &buf})
	final := pumpStreamEvents(w, ch)
	require.Empty(t, final)
	out := buf.String()
	require.Contains(t, out, "event: error")
	require.Contains(t, out, "data: boom")
}

// failingWriter errors on the Nth write. Drives pumpStreamEvents into
// its Flush-error early return.
type failingWriter struct {
	written int
	failAt  int
}

func (f *failingWriter) Write(p []byte) (int, error) {
	f.written++
	if f.written >= f.failAt {
		return 0, errors.New("disconnected")
	}
	return len(p), nil
}

// stringWriter is the simplest possible io.Writer for capturing SSE
// output in unit tests.
type stringWriter struct{ b *strings.Builder }

func (s *stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

// extractTurnID pulls the 32-char hex turn ID out of an HTML response.
var turnIDPattern = regexp.MustCompile(`/api/chat/stream/([0-9a-f]{32})`)

func extractTurnID(t *testing.T, body string) string {
	t.Helper()
	m := turnIDPattern.FindStringSubmatch(body)
	require.NotNil(t, m, "no turn ID in response body: %s", body)
	return m[1]
}
