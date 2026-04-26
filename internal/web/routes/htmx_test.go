package routes

import (
	"context"
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
		Text:  "<bad>",
		Table: &Table{Headers: []string{"col"}, Rows: [][]string{{"<row>"}}},
	})
	require.Contains(t, out, "&lt;bad&gt;")
	require.Contains(t, out, "&lt;row&gt;")
	require.NotContains(t, out, "<bad>")
	require.NotContains(t, out, "<row>")
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
}

// TestHTMX_postForm locks in that POSTed form data flows through
// formArgs into the invoker. The meals page form depends on this
// end-to-end.
func TestHTMX_postForm(t *testing.T) {
	var captured map[string]any
	app := newApp(Deps{
		InvokeTool: func(_ context.Context, _ string, raw map[string]any) (Result, error) {
			captured = raw
			return Result{Text: "ok"}, nil
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
}
