package ai

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"
)

// OllamaBootstrap orchestrates an idempotent install-and-run pipeline.
// Each step is function-typed so tests can stub them; DefaultBootstrap
// wires the real shell-out + HTTP implementations.
type OllamaBootstrap struct {
	Model   string
	BaseURL string

	HasBinary     func(ctx context.Context) bool
	InstallBinary func(ctx context.Context, w io.Writer) error
	DaemonAlive   func(ctx context.Context, baseURL string) bool
	StartDaemon   func(ctx context.Context) error
	HasModel      func(ctx context.Context, baseURL, model string) bool
	PullModel     func(ctx context.Context, baseURL, model string, w io.Writer) error

	// DaemonReadyTimeout caps the post-StartDaemon poll. Zero = 30s.
	DaemonReadyTimeout time.Duration
}

// Run installs the binary, starts the daemon, and pulls the model —
// each step skipped when its precondition holds. Progress streams to w.
//
// When BaseURL points to a non-local host the install/start/pull steps
// are skipped: bite is not the operator of someone else's daemon, so it
// just verifies reachability + model availability and surfaces a clear
// error if either is missing.
func (b OllamaBootstrap) Run(ctx context.Context, w io.Writer) error {
	if b.Model == "" {
		return errors.New("ollama bootstrap: missing Model")
	}
	base := b.BaseURL
	if base == "" {
		base = DefaultOllamaBaseURL
	}
	local := isLocalBaseURL(base)

	if local && !b.HasBinary(ctx) {
		fmt.Fprintln(w, "Installing Ollama (you may be prompted for sudo)...")
		if err := b.InstallBinary(ctx, w); err != nil {
			return fmt.Errorf("install ollama: %w", err)
		}
	}

	if !b.DaemonAlive(ctx, base) {
		if !local {
			return fmt.Errorf("ollama daemon at %s is not reachable", base)
		}
		fmt.Fprintln(w, "Starting Ollama daemon in the background...")
		if err := b.StartDaemon(ctx); err != nil {
			return fmt.Errorf("start ollama daemon: %w", err)
		}
		timeout := b.DaemonReadyTimeout
		if timeout == 0 {
			timeout = 30 * time.Second
		}
		if err := waitForOllama(ctx, b.DaemonAlive, base, timeout); err != nil {
			return err
		}
	}

	if !b.HasModel(ctx, base, b.Model) {
		if !local {
			return fmt.Errorf("ollama daemon at %s does not have model %q (run `ollama pull %s` on that host)", base, b.Model, b.Model)
		}
		fmt.Fprintf(w, "Pulling model %s (first run only — this may take a while)...\n", b.Model)
		if err := b.PullModel(ctx, base, b.Model, w); err != nil {
			return fmt.Errorf("pull model %s: %w", b.Model, err)
		}
	}
	return nil
}

// isLocalBaseURL reports whether bite owns the daemon. Anything that
// resolves to a loopback name is treated as local; everything else is
// the user's responsibility to operate.
func isLocalBaseURL(base string) bool {
	u, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := u.Hostname()
	return host == "" || host == "localhost" || host == "127.0.0.1" || host == "::1"
}

func waitForOllama(ctx context.Context, alive func(context.Context, string) bool, baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if alive(ctx, baseURL) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("ollama daemon did not become ready within %s", timeout)
}

func DefaultBootstrap(model, baseURL string) OllamaBootstrap {
	return OllamaBootstrap{
		Model:         model,
		BaseURL:       baseURL,
		HasBinary:     hasOllamaBinary,
		InstallBinary: installOllamaBinary,
		DaemonAlive:   pingOllama,
		StartDaemon:   startOllamaDaemon,
		HasModel:      hasOllamaModel,
		PullModel:     pullOllamaModel,
	}
}

func hasOllamaBinary(_ context.Context) bool {
	_, err := exec.LookPath("ollama")
	return err == nil
}

func installOllamaBinary(ctx context.Context, w io.Writer) error {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.CommandContext(ctx, "sh", "-c", "curl -fsSL https://ollama.com/install.sh | sh")
		cmd.Stdout = w
		cmd.Stderr = w
		return cmd.Run()
	case "windows":
		return errors.New("automatic install on Windows not supported — download from https://ollama.com/download")
	default:
		return fmt.Errorf("automatic install not supported on %s", runtime.GOOS)
	}
}

func pingOllama(ctx context.Context, baseURL string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

func startOllamaDaemon(_ context.Context) error {
	cmd := exec.Command("ollama", "serve")
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Start()
}

func hasOllamaModel(ctx context.Context, baseURL, model string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// /api/tags returns {"models":[{"name":"<tag>", ...}, …]}; a
	// substring probe handles both fully-qualified tags and bare names.
	return bytes.Contains(body, []byte(`"`+model+`"`)) ||
		bytes.Contains(body, []byte(`"`+model+`:`))
}

func pullOllamaModel(ctx context.Context, _ string, model string, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "ollama", "pull", model)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}
