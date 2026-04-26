package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

func TestRunChecks_allPass(t *testing.T) {
	checks := []check{
		{name: "config: load", run: func(context.Context) (string, error) {
			return "ok", nil
		}},
		{name: "db: open", run: func(context.Context) (string, error) {
			return "connected", nil
		}},
	}

	var buf bytes.Buffer
	failed := runChecks(context.Background(), &buf, checks, "✗")
	assert.Equal(t, 0, failed)
	out := buf.String()
	assert.Contains(t, out, "✓")
	assert.Contains(t, out, "config: load")
}

func TestRunChecks_oneFails(t *testing.T) {
	checks := []check{
		{name: "config: load", run: func(context.Context) (string, error) {
			return "ok", nil
		}},
		{name: "db: open", run: func(context.Context) (string, error) {
			return "", errors.New("no such file")
		}},
	}

	var buf bytes.Buffer
	failed := runChecks(context.Background(), &buf, checks, "✗")
	assert.Equal(t, 1, failed)
	assert.Contains(t, buf.String(), "no such file")
}

func TestRunChecks_allFail(t *testing.T) {
	checks := []check{
		{name: "a", run: func(context.Context) (string, error) { return "", errors.New("err-a") }},
		{name: "b", run: func(context.Context) (string, error) { return "", errors.New("err-b") }},
	}

	var buf bytes.Buffer
	failed := runChecks(context.Background(), &buf, checks, "!")
	assert.Equal(t, 2, failed)
	out := buf.String()
	assert.Contains(t, out, "err-a")
	assert.Contains(t, out, "err-b")
}

func TestRunChecks_empty(t *testing.T) {
	var buf bytes.Buffer
	failed := runChecks(context.Background(), &buf, nil, "✗")
	assert.Equal(t, 0, failed)
}

func TestRunDoctor_allPass(t *testing.T) {
	// With valid config and API key set, all hard checks pass (no ping).
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().BoolP("ping", "p", false, "ping")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctor(cmd, nil)
	if err != nil {
		t.Logf("output: %s", buf.String())
	}
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All required checks passed")
}

func TestRunDoctor_configLoadError(t *testing.T) {
	// BITE_MAX_TOKENS invalid → config.Load() fails → first hard check fails.
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().BoolP("ping", "p", false, "ping")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctor(cmd, nil)
	require.Error(t, err)
}

func TestRunDoctor_dbOpenError(t *testing.T) {
	// Valid config but bad DSN → db.Open fails → third hard check fails.
	t.Setenv("BITE_DB", "/tmp") // directory, not a file
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().BoolP("ping", "p", false, "ping")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctor(cmd, nil)
	require.Error(t, err)
}

func TestRunDoctor_withOpenAIKey(t *testing.T) {
	// OpenAI key set → soft check "media: openai key" returns success message.
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "sk-openai-fake")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().BoolP("ping", "p", false, "ping")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runDoctor(cmd, nil))
	assert.Contains(t, buf.String(), "audio transcription available")
}

func TestRunDoctor_pingFails(t *testing.T) {
	// Pre-cancelled context → ping AI call fails immediately (no network needed).
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before running — Stream will fail immediately

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.Flags().BoolP("ping", "p", false, "ping")
	require.NoError(t, cmd.Flags().Set("ping", "true"))
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctor(cmd, nil)
	// ping check fails (context cancelled) → runDoctor returns error
	if err == nil {
		t.Logf("output: %s", buf.String())
	}
	require.Error(t, err)
}

func TestRunDoctor_missingKey(t *testing.T) {
	// runDoctor with valid config but no API key → required checks fail.
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "") // no key → config: api key check fails

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().BoolP("ping", "p", false, "ping")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runDoctor(cmd, nil)
	require.Error(t, err)
	assert.Contains(t, buf.String(), "required check")
}

func TestRunDoctor_ffmpegFound(t *testing.T) {
	// Put a fake ffmpeg binary in PATH so the soft check reports success.
	fakeDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "ffmpeg"), []byte("#!/bin/sh\nexit 0"), 0o755))
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", fakeDir+":"+origPath)

	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().BoolP("ping", "p", false, "ping")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runDoctor(cmd, nil))
	assert.Contains(t, buf.String(), "video keyframe extraction available")
}

func TestPingModel_done(t *testing.T) {
	s := &mockStreamer{events: []ai.StreamEvent{
		{Delta: "pong"},
		{Done: true, Final: "pong"},
	}}
	msg, err := pingModel(context.Background(), s)
	require.NoError(t, err)
	assert.Equal(t, "model responded", msg)
}

func TestPingModel_streamError(t *testing.T) {
	s := &mockStreamer{err: errors.New("network failure")}
	_, err := pingModel(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "network failure")
}

func TestPingModel_eventError(t *testing.T) {
	s := &mockStreamer{events: []ai.StreamEvent{
		{Err: errors.New("stream blip")},
	}}
	_, err := pingModel(context.Background(), s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream blip")
}
