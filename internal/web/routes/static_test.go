package routes

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestStatic_servesEmbeddedAssets locks in that /static/* resolves to
// the embedded FS — every script and stylesheet the layout / chat page
// references must be reachable, otherwise the dashboard renders broken.
// Asserts content-type per asset too; a regression that serves CSS as
// text/plain would prevent browsers from applying it.
func TestStatic_servesEmbeddedAssets(t *testing.T) {
	app := newApp(Deps{})
	cases := []struct {
		path        string
		contentType string
	}{
		{"/static/styles.css", "text/css"},
		{"/static/htmx.min.js", "javascript"},
		{"/static/htmx-ext-sse.min.js", "javascript"},
	}
	for _, c := range cases {
		t.Run(c.path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, c.path, nil))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Contains(t, resp.Header.Get("Content-Type"), c.contentType,
				"%s must come back with the right Content-Type or browsers ignore it", c.path)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NotEmpty(t, body, "embedded asset must not be served as an empty body")
		})
	}
}

// TestStatic_missingAssetIs404 protects against a regression where a
// catch-all handler swallows unknown /static paths into a 200 with an
// empty body.
func TestStatic_missingAssetIs404(t *testing.T) {
	app := newApp(Deps{})
	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/static/does-not-exist.css", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
}
