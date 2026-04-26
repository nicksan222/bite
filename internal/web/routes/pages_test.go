package routes

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPages_render exercises every server-rendered page: each must
// resolve to 200 with text/html, a body that includes the page's
// title (proves the layout wrapped the content), and a page-specific
// piece of copy (proves the content template actually rendered — a
// regression that produces an empty content block would still pass a
// title-only check). Asserts copy text rather than CSS classes so the
// test isn't coupled to template syntax.
func TestPages_render(t *testing.T) {
	app := newApp(Deps{
		ListTools: func() []ToolMeta {
			return []ToolMeta{{Name: "meals_today", Summary: "today's intake"}}
		},
	})
	cases := []struct {
		path        string
		title       string
		contentCopy string
	}{
		{"/", "Chat", "What's on your plate?"},
		{"/dashboard", "Dashboard", "Live snapshots"},
		{"/meals", "Meals", "Log a meal"},
		{"/tools", "Tools", "Tool registry"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, c.path, nil))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "text/html; charset=utf-8", resp.Header.Get("Content-Type"))
			body, _ := io.ReadAll(resp.Body)
			got := string(body)
			require.Contains(t, got, "<title>bite — "+c.title+"</title>")
			require.Contains(t, got, c.contentCopy,
				"page-specific copy missing — content block did not render")
		})
	}
}

// TestPages_activeNavMarked asserts each page renders aria-current="page"
// on exactly the link that points back at it (and on no other). Drift
// here would silently regress screen-reader navigation.
func TestPages_activeNavMarked(t *testing.T) {
	app := newApp(Deps{
		ListTools: func() []ToolMeta { return nil },
	})
	cases := []struct {
		path  string
		label string // text inside the active <a>
	}{
		{"/", "Chat"},
		{"/dashboard", "Dashboard"},
		{"/meals", "Meals"},
		{"/tools", "Tools"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, c.path, nil))
			require.NoError(t, err)
			body, _ := io.ReadAll(resp.Body)
			got := string(body)
			// The active link must carry aria-current="page" and end
			// with the label — assert the pair so a misplaced
			// aria-current can't pass the test.
			require.Regexp(t,
				`aria-current="page"\s*>`+c.label+`</a>`,
				got,
				"expected active link with label %q to carry aria-current", c.label,
			)
			// Exactly one aria-current per page.
			require.Equal(t, 1, strings.Count(got, `aria-current="page"`),
				"more than one aria-current attribute rendered")
		})
	}
}

// TestPages_htmxConfigEnables4xxSwap pins that the layout sets the
// htmx-config meta tag so 4xx/5xx responses still swap into their
// target. Without this, every htmlError our HTMX surfaces produce
// (empty chat message, validation failures, AI not configured)
// would silently drop into htmx:responseError and the user would see
// nothing happen. We parse the meta tag's JSON so the assertion
// survives a key-order reshuffle but still locks in the contract.
func TestPages_htmxConfigEnables4xxSwap(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)

	m := regexp.MustCompile(`<meta name="htmx-config" content='([^']+)'>`).FindStringSubmatch(string(body))
	require.NotNil(t, m, "layout must declare a htmx-config meta tag")

	var cfg struct {
		ResponseHandling []struct {
			Code  string `json:"code"`
			Swap  bool   `json:"swap"`
			Error bool   `json:"error,omitempty"`
		} `json:"responseHandling"`
	}
	require.NoError(t, json.Unmarshal([]byte(m[1]), &cfg))

	var got4xx bool
	for _, rule := range cfg.ResponseHandling {
		if rule.Code == "[45].." {
			got4xx = rule.Swap
			break
		}
	}
	require.True(t, got4xx, "the [45].. rule must have swap:true so error alerts surface")
}

// TestPages_chatFormWiring locks in the chat page's load-bearing
// htmx attributes: the form must post to /api/chat, target the
// transcript with beforeend swap, and the page must load
// htmx-ext-sse so the asst bubble's sse-connect can drive the stream.
func TestPages_chatFormWiring(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	require.Contains(t, got, `hx-post="/api/chat"`,
		"chat form must post to /api/chat")
	require.Contains(t, got, `hx-target="#chat-transcript"`,
		"chat form must target the transcript so bubbles append into it")
	require.Contains(t, got, `hx-swap="beforeend"`,
		"chat form must append (not replace) so prior turns survive")
	require.Contains(t, got, `/static/htmx-ext-sse.min.js`,
		"chat page must load the SSE extension or sse-connect won't work")
	require.Contains(t, got, `hx-on:keydown=`,
		"chat input must carry a keydown handler so Enter submits (Shift+Enter newlines)")
	require.Contains(t, got, `requestSubmit()`,
		"keydown handler must call requestSubmit so form validation still runs")
	require.Contains(t, got, `getElementById('chat-empty')?.remove()`,
		"first submit must remove the empty-state welcome so it doesn't sit on top of the transcript")
	require.Contains(t, got, `event.detail.successful`,
		"after-request must reset the form ONLY on success — failed submits should leave the typed message intact")
	require.Contains(t, got, `window.scrollTo`,
		"after-swap must scroll to the bottom so the new bubble is visible")
}

// TestPages_mealsFormWiring locks in the load-bearing attributes on
// the meals page log-form: it must hx-post to /htmx/tool/log_meal,
// reset on success, and dispatch a body-level "refresh" event so the
// "Today" card updates immediately. A regression in any of these
// breaks the meals UX without a visible error.
func TestPages_mealsFormWiring(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/meals", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	require.Contains(t, got, `hx-post="/htmx/tool/log_meal"`,
		"meals form must post to the log_meal tool")
	require.Contains(t, got, `dispatchEvent(new Event('refresh'))`,
		"meals form must dispatch refresh after success so the Today card updates")
	require.Contains(t, got, `refresh from:body`,
		"the Today card must subscribe to the body-level refresh event")
}

// TestPages_dashboardRendersCards proves the dashboard's {{range
// .Cards}} loop actually fires. Without this, a regression that
// silently drops the loop (or feeds an empty Cards slice) would still
// pass TestPages_render — only the page header is asserted there.
func TestPages_dashboardRendersCards(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	for _, card := range dashboardCards {
		require.Contains(t, got, card.Title,
			"dashboard must render every card's title — %q is missing", card.Title)
		require.Contains(t, got, "/htmx/tool/"+card.Tool,
			"dashboard must wire each card's hx-get to /htmx/tool/%s", card.Tool)
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

// TestPages_toolsRendersParamBadges proves the tools page renders each
// param with the required/optional distinction visible in the badge
// markup. Without this, a regression that drops the required check
// (or the params loop entirely) would still pass the name+summary
// assertions in TestPages_toolsListsRegistry.
func TestPages_toolsRendersParamBadges(t *testing.T) {
	app := newApp(Deps{
		ListTools: func() []ToolMeta {
			return []ToolMeta{{
				Name:    "log_meal",
				Summary: "logs a meal",
				Params: []ParamMeta{
					{Name: "title", Type: "string", Required: true},
					{Name: "kcal", Type: "float"},
				},
			}}
		},
	})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	got := string(body)
	require.Contains(t, got, "title")
	require.Contains(t, got, "kcal")
	require.Contains(t, got, "badge-warning",
		"required params must wear the warning badge")
	require.Contains(t, got, "badge-outline",
		"optional params must wear the outline badge")
	require.Contains(t, got, `title="required string"`,
		"required-param hover hint must spell out the type")
	require.Contains(t, got, `title="optional float"`,
		"optional-param hover hint must spell out the type")
}

// TestPages_toolsHandlesNilDeps ensures the page degrades gracefully
// without a configured ListTools (empty registry, not a crash).
func TestPages_toolsHandlesNilDeps(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/tools", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
