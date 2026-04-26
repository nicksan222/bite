package routes

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// newApp builds a fiber.App wired with the given Deps. Tests drive
// handlers through app.Test() so no port is bound.
func newApp(d Deps) *fiber.App {
	app := fiber.New()
	Register(app, d)
	return app
}

// stubDeps supplies a single fixture tool. Pass a custom invoke to
// override behavior; nil routes invokes back to a no-op.
func stubDeps(invoke func(ctx context.Context, name string, raw map[string]any) (Result, error)) Deps {
	return Deps{
		ListTools: func() []ToolMeta {
			return []ToolMeta{{Name: "echo", Summary: "echo back", Params: []ParamMeta{{Name: "msg", Type: "string"}}}}
		},
		InvokeTool: invoke,
	}
}

func TestAPI_listTools(t *testing.T) {
	app := newApp(stubDeps(nil))
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/tools", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var body struct {
		Tools []ToolMeta `json:"tools"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	require.Len(t, body.Tools, 1)
	require.Equal(t, "echo", body.Tools[0].Name)
}

func TestAPI_invokeTool_success(t *testing.T) {
	app := newApp(stubDeps(func(_ context.Context, name string, raw map[string]any) (Result, error) {
		require.Equal(t, "echo", name)
		require.Equal(t, "hi", raw["msg"])
		return Result{Text: "hi"}, nil
	}))
	resp, err := app.Test(postJSON("/api/tools/echo", `{"msg":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got Result
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&got))
	require.Equal(t, "hi", got.Text)
}

func TestAPI_invokeTool_emptyBodyOK(t *testing.T) {
	// Zero-arg tools must be invokable with no body — the dashboard's
	// "Refresh" buttons rely on this.
	called := false
	app := newApp(stubDeps(func(_ context.Context, _ string, raw map[string]any) (Result, error) {
		called = true
		require.Empty(t, raw)
		return Result{Text: "ok"}, nil
	}))
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/api/tools/echo", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.True(t, called)
}

func TestAPI_invokeTool_notFound(t *testing.T) {
	app := newApp(stubDeps(func(_ context.Context, name string, _ map[string]any) (Result, error) {
		return Result{}, NotFoundError{Name: name}
	}))
	resp, err := app.Test(postJSON("/api/tools/missing", `{}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestAPI_invokeTool_runError(t *testing.T) {
	app := newApp(stubDeps(func(_ context.Context, _ string, _ map[string]any) (Result, error) {
		return Result{}, errors.New("boom")
	}))
	resp, err := app.Test(postJSON("/api/tools/echo", `{}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAPI_invokeTool_badJSON(t *testing.T) {
	app := newApp(stubDeps(func(_ context.Context, _ string, _ map[string]any) (Result, error) {
		t.Fatal("invoker must not be called on parse failure")
		return Result{}, nil
	}))
	resp, err := app.Test(postJSON("/api/tools/echo", "not json"))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestAPI_unconfiguredDeps locks in the contract that every surface
// returns 503 (not 500, not 404) when its required Deps closure is
// missing. Catches a regression where a new handler forgets the nil-check.
func TestAPI_unconfiguredDeps(t *testing.T) {
	app := newApp(Deps{})
	cases := []struct {
		name string
		req  *http.Request
	}{
		{"GET /api/tools", httptest.NewRequest(http.MethodGet, "/api/tools", nil)},
		{"POST /api/tools/:name", httptest.NewRequest(http.MethodPost, "/api/tools/x", nil)},
		{"POST /api/chat", postJSON("/api/chat", `{"message":"hi"}`)},
		{"GET /htmx/tool/:name", httptest.NewRequest(http.MethodGet, "/htmx/tool/x", nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := app.Test(c.req)
			require.NoError(t, err)
			require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
		})
	}
}

// stubStreamer returns the configured deltas, then either a terminal
// error or a terminating Done. Used by chat tests that need to drive the
// SSE pipeline without a real model.
type stubStreamer struct {
	deltas   []string
	final    string
	streamer error // returned synchronously from Stream — simulates handshake failure
	tail     error // emitted as a terminal ev.Err — simulates mid-stream failure
}

func (s stubStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
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

func TestChat_emptyMessage(t *testing.T) {
	app := newApp(Deps{AI: stubStreamer{}})
	resp, err := app.Test(postJSON("/api/chat", `{"message":"   "}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestChat_badJSON(t *testing.T) {
	app := newApp(Deps{AI: stubStreamer{}})
	resp, err := app.Test(postJSON("/api/chat", "not json"))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestChat_streamRoundtrip exercises the full SSE encoding: deltas land
// as `event: delta` blocks and channel-close-with-Done lands as a
// terminating `event: done` block. Browser clients depend on this shape.
func TestChat_streamRoundtrip(t *testing.T) {
	app := newApp(Deps{
		AI: stubStreamer{deltas: []string{"hi", " there"}, final: "hi there"},
	})
	resp, err := app.Test(postJSON("/api/chat", `{"message":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	got := string(body)
	require.Contains(t, got, "event: delta")
	require.Contains(t, got, `"text":"hi"`)
	require.Contains(t, got, `"text":" there"`)
	require.Contains(t, got, "event: done")
	require.Contains(t, got, `"final":"hi there"`)
}

// TestChat_streamErrorEvent locks in the mid-stream failure shape:
// when Stream emits ev.Err, the response body must contain an SSE
// "event: error" block carrying the error message. Browser clients
// rely on this to surface the failure in the chat-error banner.
func TestChat_streamErrorEvent(t *testing.T) {
	app := newApp(Deps{
		AI: stubStreamer{deltas: []string{"part"}, tail: errors.New("model exploded")},
	})
	resp, err := app.Test(postJSON("/api/chat", `{"message":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	got := string(body)
	require.Contains(t, got, "event: error")
	require.Contains(t, got, `"message":"model exploded"`)
	// Mid-stream failure should NOT also emit a terminating "done" event —
	// otherwise the client double-handles the turn.
	require.NotContains(t, got, "event: done")
}

// TestChat_streamHandshakeError covers the handshake-failure branch:
// when Stream itself returns an error (no channel created) the response
// must be a JSON 502 — not an SSE stream — so the browser fetch sees
// .ok === false and routes through the catch path.
func TestChat_streamHandshakeError(t *testing.T) {
	app := newApp(Deps{
		AI: stubStreamer{streamer: errors.New("upstream down")},
	})
	resp, err := app.Test(postJSON("/api/chat", `{"message":"hi"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadGateway, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

func postJSON(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}
