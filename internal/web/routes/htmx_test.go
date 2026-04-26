package routes

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// renderResultHTML's escape behavior is the load-bearing security
// contract — tool output flows directly into the DOM via hx-swap.
// Structure-level assertions are deliberately loose; the escape ones
// are exact.
func TestHTMX_renderResultHTML_escapes(t *testing.T) {
	out := renderResultHTML(Result{
		Text: "<bad>",
		Table: &Table{
			Headers: []string{"<h>"},
			Rows:    [][]string{{"<row>"}},
			Footer:  []string{"<foot>"},
		},
	})
	for _, raw := range []string{"<bad>", "<h>", "<row>", "<foot>"} {
		require.NotContains(t, out, raw, "raw HTML must never reach the swapped fragment")
	}
	for _, escaped := range []string{"&lt;bad&gt;", "&lt;h&gt;", "&lt;row&gt;", "&lt;foot&gt;"} {
		require.Contains(t, out, escaped)
	}
}

// TestHTMX_renderResultHTML_emptyResult locks in the degenerate cases:
// no fields → an empty wrapper div, never a stray <pre> or <table>.
func TestHTMX_renderResultHTML_emptyResult(t *testing.T) {
	require.Equal(t, "<div></div>", renderResultHTML(Result{}))
}

// TestHTMX_renderResultHTML_tableOnly proves the text branch is optional
// — earlier hand-rolled code emitted <pre></pre> on an empty Text. The
// template form must NOT.
func TestHTMX_renderResultHTML_tableOnly(t *testing.T) {
	out := renderResultHTML(Result{
		Table: &Table{Headers: []string{"a"}, Rows: [][]string{{"1"}}},
	})
	require.NotContains(t, out, "<pre")
	require.Contains(t, out, "<table")
	require.Contains(t, out, "<th>a</th>")
	require.Contains(t, out, "<td>1</td>")
}

// TestHTMX_htmlAlert_escapes pins that the alert helper escapes its
// payload — error messages can include arbitrary tool output.
func TestHTMX_htmlAlert_escapes(t *testing.T) {
	out := htmlAlert(`<script>alert(1)</script>`)
	require.Contains(t, out, `alert alert-error`)
	require.NotContains(t, out, `<script>`)
	require.Contains(t, out, `&lt;script&gt;`)
}

// TestHTMX_tool_runErrorReturnsHTMLAlert covers the non-NotFound error
// branch — an arbitrary tool failure must surface as a 400 with an HTML
// alert fragment (not JSON), since the response is hx-swapped into the
// DOM.
func TestHTMX_tool_runErrorReturnsHTMLAlert(t *testing.T) {
	app := newApp(Deps{
		InvokeTool: func(_ context.Context, _ string, _ map[string]any) (Result, error) {
			return Result{}, errors.New("kcal must be positive")
		},
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/htmx/tool/log_meal", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `alert alert-error`)
	require.Contains(t, string(body), `kcal must be positive`)
}

// TestHTMX_unconfiguredDepsReturnsHTML pins that an HTMX endpoint never
// returns JSON when its hx-target is expecting an HTML fragment — the
// 503 case must still produce an alert markup, not a JSON envelope that
// would render as raw text inside the swapped element.
func TestHTMX_unconfiguredDepsReturnsHTML(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/htmx/tool/x", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `alert alert-error`)
	require.Contains(t, string(body), "tool invocation not configured")
}

func TestHTMX_tool_endpoint(t *testing.T) {
	app := newApp(Deps{
		InvokeTool: func(_ context.Context, name string, raw map[string]any) (Result, error) {
			require.Equal(t, "echo", name)
			require.Equal(t, "hi", raw["msg"])
			return Result{Text: "ok"}, nil
		},
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/htmx/tool/echo?msg=hi", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "ok")
}

func TestHTMX_tool_notFound(t *testing.T) {
	app := newApp(Deps{
		InvokeTool: func(_ context.Context, name string, _ map[string]any) (Result, error) {
			return Result{}, NotFoundError{Name: name}
		},
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/htmx/tool/nope", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "text/html")
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), `alert alert-error`)
	require.Contains(t, string(body), "tool not found: nope",
		"404 alert must echo the requested tool name so the user knows what failed")
}

// TestHTMX_postForm locks in that POSTed form data flows through
// formArgs into the invoker. The meals page form depends on this
// end-to-end.
func TestHTMX_postForm(t *testing.T) {
	var captured map[string]any
	app := newApp(Deps{
		InvokeTool: func(_ context.Context, _ string, raw map[string]any) (Result, error) {
			captured = raw
			return Result{Text: "logged"}, nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/htmx/tool/log_meal",
		strings.NewReader("title=salad&kcal=300"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "salad", captured["title"])
	require.Equal(t, "300", captured["kcal"], "form values arrive as strings; the registry adapter coerces upstream")
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "logged",
		"rendered fragment must echo the tool's Result.Text — the meals page hx-target depends on this")
}
