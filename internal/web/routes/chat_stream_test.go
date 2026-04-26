package routes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
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
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html",
		"validation errors must render as HTML alerts so the form's hx-target shows them")
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "empty message")
	require.Contains(t, string(body), `alert alert-error`)
}

func TestChatStart_unconfigured(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html",
		"unconfigured AI must surface as an HTML alert (the form's hx-target is the transcript)")
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "AI not configured")
	require.Contains(t, string(body), `alert alert-error`)
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
	require.Regexp(t, regexp.MustCompile(`sse-connect="`+chatStreamPathPrefix+`[0-9a-f]{32}"`), got)
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

	resp, err = app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+turnID, nil))
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
// only be popped once. A second GET still gets a 200 SSE stream (the
// always-SSE rule) but with an `event: error` payload so the asst
// bubble can surface "turn expired or not found" instead of replaying
// the conversation on a browser refresh.
func TestChatTurn_idIsSingleUse(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{deltas: []string{"x"}, final: "x"}})

	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	turnID := extractTurnID(t, string(body))

	first, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+turnID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, first.StatusCode)
	_, _ = io.ReadAll(first.Body)

	second, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+turnID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, second.StatusCode, "stream endpoint always returns 200; the failure rides as an SSE error event")
	body2, _ := io.ReadAll(second.Body)
	require.Contains(t, string(body2), "event: error")
	require.Contains(t, string(body2), "turn expired or not found")
}

// TestChatTurn_unknownIdYieldsSSEError exercises the cold path: a GET
// for an ID that was never stashed (link forge, restart, etc) lands on
// a 200 SSE stream that emits exactly one error event so the asst
// bubble can surface the failure rather than getting stuck.
func TestChatTurn_unknownIdYieldsSSEError(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{}})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+"deadbeef", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "event: error")
	require.Contains(t, string(body), "event: done")
}

// TestChatStream_unconfiguredYieldsSSEError covers the AI-not-configured
// path: instead of a 503 JSON, the user sees an SSE error event in the
// asst bubble, plus a terminating done event so htmx-ext-sse closes
// the EventSource cleanly via sse-close="done".
func TestChatStream_unconfiguredYieldsSSEError(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+"whatever", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	require.Contains(t, got, "event: error")
	require.Contains(t, got, "AI not configured")
	require.Contains(t, got, "event: done",
		"every error path must terminate with a done event so the EventSource closes cleanly")
}

// TestChatStream_forwardsStreamOpts proves the chat handler still
// invokes Deps.StreamOpts() and threads the returned options into
// ai.Streamer.Stream — the seam through which the chat tool wires up
// tool-calls.
func TestChatStream_forwardsStreamOpts(t *testing.T) {
	streamer := &stubStreamer{deltas: []string{"ok"}, final: "ok"}
	calls := 0
	app := newApp(Deps{
		AI: streamer,
		StreamOpts: func() []ai.StreamOption {
			calls++
			return []ai.StreamOption{ai.WithSystemPrompt("be terse"), ai.WithSystemPrompt("be specific")}
		},
	})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	turnID := extractTurnID(t, string(body))

	streamResp, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+turnID, nil))
	require.NoError(t, err)
	_, _ = io.ReadAll(streamResp.Body)
	require.Equal(t, 1, calls, "StreamOpts must be invoked exactly once per stream")
	require.Equal(t, 2, streamer.gotOptCount, "every option StreamOpts returned must reach Stream")
}

// TestChatStream_handshakeErrorYieldsSSEError covers the branch where
// Stream() itself errors before a channel is opened — the failure
// surfaces as an SSE error event so the user sees what went wrong
// instead of staring at a stuck spinner.
func TestChatStream_handshakeErrorYieldsSSEError(t *testing.T) {
	app := newApp(Deps{AI: &stubStreamer{streamer: errors.New("upstream down")}})
	resp, err := app.Test(postForm("/api/chat", map[string]string{"message": "hi"}))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	turnID := extractTurnID(t, string(body))

	streamResp, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+turnID, nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, streamResp.StatusCode)
	streamBody, _ := io.ReadAll(streamResp.Body)
	require.Contains(t, string(streamBody), "event: error")
	require.Contains(t, string(streamBody), "upstream down")
}

// TestChatTurn_sessionAccumulates: the second turn's history must
// include the assistant reply from the first turn. Proves the session
// store is wired and the post-stream history append actually happens.
func TestChatTurn_sessionAccumulates(t *testing.T) {
	streamer := &stubStreamer{deltas: []string{"answer"}, final: "answer"}
	app := newApp(Deps{AI: streamer})

	// Turn 1: post then drain the stream so the asst reply hits the
	// session history.
	cookie := runChatTurn(t, app, nil, "first")
	require.NotNil(t, cookie, "first POST must set the session cookie")

	// Turn 2: reuse the cookie. By the time runChatTurn pops the turn
	// off the stash, ai.Streamer sees the full prior context.
	runChatTurn(t, app, cookie, "second")

	require.Len(t, streamer.gotMessages, 3,
		"second turn should carry the prior user+asst plus the new user turn")
	require.Equal(t, []ai.Role{ai.RoleUser, ai.RoleAssistant, ai.RoleUser},
		[]ai.Role{streamer.gotMessages[0].Role, streamer.gotMessages[1].Role, streamer.gotMessages[2].Role})
}

// runChatTurn POSTs a message, drains the resulting SSE stream, and
// returns the session cookie set by the response (or the cookie passed
// in, whichever is fresher). Centralises the four-line dance every
// session test was repeating.
func runChatTurn(t *testing.T, app *fiber.App, cookie *http.Cookie, message string) *http.Cookie {
	t.Helper()
	req := postForm("/api/chat", map[string]string{"message": message})
	if cookie != nil {
		req.AddCookie(cookie)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	turnID := extractTurnID(t, string(body))

	streamResp, err := app.Test(httptest.NewRequest(http.MethodGet, chatStreamPathPrefix+turnID, nil))
	require.NoError(t, err)
	_, _ = io.ReadAll(streamResp.Body)

	if cs := resp.Cookies(); len(cs) > 0 {
		return cs[0]
	}
	return cookie
}

// extractTurnID pulls the 32-char hex turn ID out of an HTML response.
var turnIDPattern = regexp.MustCompile(chatStreamPathPrefix + `([0-9a-f]{32})`)

func extractTurnID(t *testing.T, body string) string {
	t.Helper()
	m := turnIDPattern.FindStringSubmatch(body)
	require.NotNil(t, m, "no turn ID in response body: %s", body)
	return m[1]
}
