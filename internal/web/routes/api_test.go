package routes

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
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

// postJSON is the JSON-body POST request fixture every API test reaches
// for. Lives here next to newApp so all surface tests share one
// construction path.
func postJSON(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
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
	requireJSONError(t, resp, http.StatusNotFound, "tool not found: missing")
}

func TestAPI_invokeTool_runError(t *testing.T) {
	app := newApp(stubDeps(func(_ context.Context, _ string, _ map[string]any) (Result, error) {
		return Result{}, errors.New("boom")
	}))
	resp, err := app.Test(postJSON("/api/tools/echo", `{}`))
	require.NoError(t, err)
	requireJSONError(t, resp, http.StatusBadRequest, "boom")
}

func TestAPI_invokeTool_badJSON(t *testing.T) {
	app := newApp(stubDeps(func(_ context.Context, _ string, _ map[string]any) (Result, error) {
		t.Fatal("invoker must not be called on parse failure")
		return Result{}, nil
	}))
	resp, err := app.Test(postJSON("/api/tools/echo", "not json"))
	require.NoError(t, err)
	requireJSONError(t, resp, http.StatusBadRequest, "invalid json")
}

// requireJSONError asserts the JSON-error envelope every API surface
// returns: the right status, application/json content type, and an
// "error" field that contains wantSubstr. Centralised so a future
// envelope tweak (e.g. adding a "code" field) is one edit.
func requireJSONError(t *testing.T, resp *http.Response, status int, wantSubstr string) {
	t.Helper()
	require.Equal(t, status, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
	var env struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&env))
	require.Contains(t, env.Error, wantSubstr,
		"error envelope must carry an actionable message — %q is missing %q", env.Error, wantSubstr)
}

// TestAPI_unconfiguredDeps locks in the contract that every surface
// returns 503 (not 500, not 404) when its required Deps closure is
// missing, AND that each surface emits its native envelope (JSON for
// the JSON API, HTML alert for the HTMX surfaces). Catches both
// nil-check regressions and content-type drift.
func TestAPI_unconfiguredDeps(t *testing.T) {
	app := newApp(Deps{})
	cases := []struct {
		name        string
		req         *http.Request
		contentType string
	}{
		{"GET /api/tools", httptest.NewRequest(http.MethodGet, "/api/tools", nil), "application/json"},
		{"POST /api/tools/:name", httptest.NewRequest(http.MethodPost, "/api/tools/x", nil), "application/json"},
		{"POST /api/chat", postJSON("/api/chat", `{"message":"hi"}`), "text/html"},
		{"GET /htmx/tool/:name", httptest.NewRequest(http.MethodGet, "/htmx/tool/x", nil), "text/html"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, err := app.Test(c.req)
			require.NoError(t, err)
			require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
			require.Contains(t, resp.Header.Get("Content-Type"), c.contentType,
				"each surface must emit its native envelope on 503, not coerce to JSON")
		})
	}
}
