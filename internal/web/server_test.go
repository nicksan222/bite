package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNew_endToEnd is the integration test for web.New: a full request
// flows through the configured fiber.App (middleware + routes) and the
// expected status comes back. Per-handler behavior is tested in routes/;
// this catches regressions in the wrapper itself — middleware wiring,
// type-alias drift, etc.
func TestNew_endToEnd(t *testing.T) {
	srv := New(Deps{
		ListTools: func() []ToolMeta {
			return []ToolMeta{{Name: "echo", Summary: "echo"}}
		},
	})
	resp, err := srv.App().Test(httptest.NewRequest(http.MethodGet, "/api/tools", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// requestid middleware must tag every response — proves the middleware
	// chain in New is intact.
	require.NotEmpty(t, resp.Header.Get("X-Request-Id"))
}

// TestConfig_Addr pins the listener defaults: zero values produce a
// loopback-only bind, deliberately, so an operator opts in to LAN
// exposure rather than getting it by accident.
func TestConfig_Addr(t *testing.T) {
	require.Equal(t, "127.0.0.1:8787", Config{}.Addr())
	require.Equal(t, "0.0.0.0:9000", Config{Host: "0.0.0.0", Port: 9000}.Addr())
}
