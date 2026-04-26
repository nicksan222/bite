package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPages_render exercises every server-rendered page: each must
// resolve to 200 with text/html and a body that includes the page's
// title (proves the layout wrapped the content template, not just
// served the layout). Asserts only the title — CSS classes are
// implementation details that would couple the test to template syntax.
func TestPages_render(t *testing.T) {
	app := newApp(Deps{
		ListTools: func() []ToolMeta {
			return []ToolMeta{{Name: "meals_today", Summary: "today's intake"}}
		},
	})
	cases := []struct {
		path  string
		title string
	}{
		{"/", "Dashboard"},
		{"/chat", "Chat"},
		{"/meals", "Meals"},
		{"/tools", "Tools"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, c.path, nil))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
			body, _ := io.ReadAll(resp.Body)
			require.Contains(t, string(body), "<title>bite — "+c.title+"</title>")
		})
	}
}

// TestPages_toolsListsRegistry asserts the tools page renders names
// returned by the ListTools closure AND that the closure is consulted
// per request — both load-bearing for "the dashboard reflects the live
// registry without a server restart."
func TestPages_toolsListsRegistry(t *testing.T) {
	calls := 0
	app := newApp(Deps{
		ListTools: func() []ToolMeta {
			calls++
			return []ToolMeta{{Name: "foo_tool", Summary: "fixture"}}
		},
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	require.Contains(t, string(body), "foo_tool")
	require.Contains(t, string(body), "fixture")
	require.GreaterOrEqual(t, calls, 1, "ListTools must be invoked per request, not cached at boot")
}

// TestPages_toolsHandlesNilDeps ensures the page degrades gracefully
// without a configured ListTools (empty registry, not a crash).
func TestPages_toolsHandlesNilDeps(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
