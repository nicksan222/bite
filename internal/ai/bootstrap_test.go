package ai

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubBootstrap returns an OllamaBootstrap whose every step is a stub
// recording its call and returning success. Tests override individual
// fields to flip outcomes.
func stubBootstrap() *OllamaBootstrap {
	return &OllamaBootstrap{
		Model:         "qwen3:4b",
		BaseURL:       "http://localhost:11434",
		HasBinary:     func(context.Context) bool { return true },
		InstallBinary: func(context.Context, io.Writer) error { return nil },
		DaemonAlive:   func(context.Context, string) bool { return true },
		StartDaemon:   func(context.Context) error { return nil },
		HasModel:      func(context.Context, string, string) bool { return true },
		PullModel:     func(context.Context, string, string, io.Writer) error { return nil },
	}
}

func TestRun_allPreconditionsMet_writesNothing(t *testing.T) {
	b := stubBootstrap()
	var w bytes.Buffer
	require.NoError(t, b.Run(context.Background(), &w))
	assert.Empty(t, w.String(), "every step short-circuits when preconditions are met")
}

func TestRun_missingModelErrors(t *testing.T) {
	b := stubBootstrap()
	b.Model = ""
	require.Error(t, b.Run(context.Background(), io.Discard))
}

func TestRun_installsWhenBinaryMissing(t *testing.T) {
	b := stubBootstrap()
	b.HasBinary = func(context.Context) bool { return false }
	installed := false
	b.InstallBinary = func(_ context.Context, w io.Writer) error {
		installed = true
		_, _ = w.Write([]byte("installer log\n"))
		return nil
	}
	var w bytes.Buffer
	require.NoError(t, b.Run(context.Background(), &w))
	assert.True(t, installed)
	assert.Contains(t, w.String(), "Installing Ollama")
	assert.Contains(t, w.String(), "installer log")
}

func TestRun_installFailureSurfaced(t *testing.T) {
	b := stubBootstrap()
	b.HasBinary = func(context.Context) bool { return false }
	b.InstallBinary = func(context.Context, io.Writer) error { return errors.New("network down") }
	err := b.Run(context.Background(), io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "install ollama")
	assert.ErrorContains(t, err, "network down")
}

func TestRun_startsDaemonWhenDown(t *testing.T) {
	b := stubBootstrap()
	calls := 0
	b.DaemonAlive = func(context.Context, string) bool {
		calls++
		return calls > 1 // dead on first probe, alive on the polled retry
	}
	started := false
	b.StartDaemon = func(context.Context) error { started = true; return nil }
	var w bytes.Buffer
	require.NoError(t, b.Run(context.Background(), &w))
	assert.True(t, started)
	assert.Contains(t, w.String(), "Starting Ollama daemon")
}

func TestRun_daemonNeverReadyTimesOut(t *testing.T) {
	b := stubBootstrap()
	b.DaemonAlive = func(context.Context, string) bool { return false }
	b.StartDaemon = func(context.Context) error { return nil }
	b.DaemonReadyTimeout = 50 * time.Millisecond
	err := b.Run(context.Background(), io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "did not become ready")
}

func TestRun_pullsModelWhenMissing(t *testing.T) {
	b := stubBootstrap()
	b.HasModel = func(context.Context, string, string) bool { return false }
	pulled := ""
	b.PullModel = func(_ context.Context, _ string, model string, w io.Writer) error {
		pulled = model
		_, _ = w.Write([]byte("pulling layers\n"))
		return nil
	}
	var w bytes.Buffer
	require.NoError(t, b.Run(context.Background(), &w))
	assert.Equal(t, "qwen3:4b", pulled)
	assert.Contains(t, w.String(), "Pulling model qwen3:4b")
}

func TestRun_pullFailureNamesModel(t *testing.T) {
	b := stubBootstrap()
	b.HasModel = func(context.Context, string, string) bool { return false }
	b.PullModel = func(context.Context, string, string, io.Writer) error { return errors.New("disk full") }
	err := b.Run(context.Background(), io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "pull model qwen3:4b")
	assert.ErrorContains(t, err, "disk full")
}

func TestRun_canceledContextStopsWait(t *testing.T) {
	b := stubBootstrap()
	b.DaemonAlive = func(context.Context, string) bool { return false }
	b.StartDaemon = func(context.Context) error { return nil }
	b.DaemonReadyTimeout = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := b.Run(ctx, io.Discard)
	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "context canceled"))
}

// ─── remote BaseURL behaviour ───────────────────────────────────────────────

func TestRun_remoteSkipsLocalInstallAndStart(t *testing.T) {
	b := stubBootstrap()
	b.BaseURL = "http://gpu-box:11434"
	installed, started := false, false
	b.HasBinary = func(context.Context) bool {
		t.Errorf("HasBinary must not be probed for a remote daemon")
		return false
	}
	b.InstallBinary = func(context.Context, io.Writer) error { installed = true; return nil }
	b.StartDaemon = func(context.Context) error { started = true; return nil }

	require.NoError(t, b.Run(context.Background(), io.Discard))
	assert.False(t, installed, "remote daemon means user-managed install")
	assert.False(t, started, "remote daemon means user-managed lifecycle")
}

func TestRun_remoteFailsWhenDaemonUnreachable(t *testing.T) {
	b := stubBootstrap()
	b.BaseURL = "http://gpu-box:11434"
	b.DaemonAlive = func(context.Context, string) bool { return false }
	b.StartDaemon = func(context.Context) error {
		t.Errorf("StartDaemon must not run for a remote daemon")
		return nil
	}

	err := b.Run(context.Background(), io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "gpu-box")
	assert.ErrorContains(t, err, "not reachable")
}

func TestRun_remoteFailsWithGuidanceWhenModelMissing(t *testing.T) {
	b := stubBootstrap()
	b.BaseURL = "http://gpu-box:11434"
	b.HasModel = func(context.Context, string, string) bool { return false }
	b.PullModel = func(context.Context, string, string, io.Writer) error {
		t.Errorf("PullModel must not run for a remote daemon")
		return nil
	}

	err := b.Run(context.Background(), io.Discard)
	require.Error(t, err)
	assert.ErrorContains(t, err, "ollama pull qwen3:4b")
}

// ─── isLocalBaseURL ─────────────────────────────────────────────────────────

func TestIsLocalBaseURL(t *testing.T) {
	cases := map[string]bool{
		"http://localhost:11434":     true,
		"http://localhost":           true,
		"http://127.0.0.1:11434":     true,
		"http://[::1]:11434":         true,
		"https://localhost:8443":     true,
		"http://gpu-box:11434":       false,
		"http://192.168.1.10:11434":  false,
		"https://ollama.example.com": false,
		"":                           true,
	}
	for url, want := range cases {
		assert.Equal(t, want, isLocalBaseURL(url), "url=%q", url)
	}
}

func TestDefaultBootstrap_wiresAllSteps(t *testing.T) {
	b := DefaultBootstrap("qwen3:4b", "http://example:11434")
	assert.Equal(t, "qwen3:4b", b.Model)
	assert.Equal(t, "http://example:11434", b.BaseURL)
	assert.NotNil(t, b.HasBinary)
	assert.NotNil(t, b.InstallBinary)
	assert.NotNil(t, b.DaemonAlive)
	assert.NotNil(t, b.StartDaemon)
	assert.NotNil(t, b.HasModel)
	assert.NotNil(t, b.PullModel)
}
