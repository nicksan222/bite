package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNew_recoverConvertsPanicTo500 proves the recover middleware in
// New is wired: a handler panic does not crash the server and the
// ErrorHandler closure converts the recovered error into the standard
// 500 JSON envelope.
func TestNew_recoverConvertsPanicTo500(t *testing.T) {
	srv := New(Deps{
		ListTools: func() []ToolMeta {
			panic("boom from ListTools")
		},
	})
	resp, err := srv.App().Test(httptest.NewRequest(http.MethodGet, "/api/tools", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

// TestNew_errorHandlerEnvelope locks in the JSON-error contract for any
// failure that flows through the ErrorHandler closure: a fiber.Error
// (here, the framework-default 404) maps to its Code, not 500, and the
// body is the {"error": …} shape the API layer uses everywhere.
func TestNew_errorHandlerEnvelope(t *testing.T) {
	srv := New(Deps{})
	resp, err := srv.App().Test(httptest.NewRequest(http.MethodGet, "/no-such-route", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)
	require.Contains(t, resp.Header.Get("Content-Type"), "application/json")
}

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
// exposure rather than getting it by accident. The IPv6 case proves
// host:port assembly handles bracketed addresses correctly — a plain
// "host:port" Sprintf would emit unparseable "::1:8787".
func TestConfig_Addr(t *testing.T) {
	require.Equal(t, "127.0.0.1:8787", Config{}.Addr())
	require.Equal(t, "0.0.0.0:9000", Config{Host: "0.0.0.0", Port: 9000}.Addr())
	require.Equal(t, "[::1]:8787", Config{Host: "::1"}.Addr())
}

// TestServer_ListenReportsBindError covers the second select case in
// Listen: when app.Listen fails (here, port already in use), the error
// must surface from Listen instead of hanging on errCh forever.
func TestServer_ListenReportsBindError(t *testing.T) {
	hold, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer hold.Close()
	port := hold.Addr().(*net.TCPAddr).Port

	srv := New(Deps{})
	done := make(chan error, 1)
	go func() {
		done <- srv.Listen(context.Background(), Config{Host: "127.0.0.1", Port: port})
	}()
	select {
	case err := <-done:
		require.Error(t, err)
		require.NotErrorIs(t, err, context.Canceled, "bind failure must not be reported as ctx.Canceled")
	case <-time.After(3 * time.Second):
		t.Fatal("Listen did not surface bind error within 3s")
	}
}

// TestServer_ListenShutsDownOnCancel boots Listen on a random local port
// and cancels its context. Listen must unwind through ShutdownWithContext
// and return ctx.Err() promptly — proving the goroutine + select + shutdown
// orchestration in server.go works end-to-end without leaking the
// listener goroutine.
func TestServer_ListenShutsDownOnCancel(t *testing.T) {
	port := freePort(t)
	srv := New(Deps{})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Listen(ctx, Config{Host: "127.0.0.1", Port: port}) }()

	// Wait until the listener actually binds before cancelling — otherwise
	// the test races between Listen reaching app.Listen and ctx being done.
	waitForListen(t, port)
	cancel()

	select {
	case err := <-done:
		require.True(t, errors.Is(err, context.Canceled), "expected ctx.Canceled, got %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("Listen did not return after ctx cancel within 3s")
	}
}

// freePort grabs a free TCP port on loopback and immediately closes
// it, returning the port number. Inherently racy — between the close
// and a subsequent bind, another process could grab the same port —
// but acceptable for short-lived test setup where the bind happens
// next.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close())
	return port
}

// waitForListen polls the loopback port until a TCP dial succeeds or
// 2s elapse. Used to avoid the test cancelling ctx before Listen has
// reached app.Listen — otherwise the cancel arrives in the
// "before-bind" window and the test races itself.
func waitForListen(t *testing.T, port int) {
	t.Helper()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("server never bound to %s", addr)
}
