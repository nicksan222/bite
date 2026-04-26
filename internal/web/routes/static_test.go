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
func TestStatic_servesEmbeddedAssets(t *testing.T) {
	app := newApp(Deps{})
	for _, path := range []string{"/static/styles.css", "/static/htmx.min.js", "/static/htmx-ext-sse.min.js"} {
		t.Run(path, func(t *testing.T) {
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)
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
