package tools

import (
	"context"
	"maps"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubChatEnv sets every env var the chat-launch path reads to sane defaults;
// pass overrides to flip individual values (e.g. an empty key, a bad DSN).
func stubChatEnv(t *testing.T, overrides map[string]string) {
	t.Helper()
	defaults := map[string]string{
		"BITE_DB":           t.TempDir() + "/test.db",
		"BITE_MODEL":        "claude-haiku-4-5",
		"BITE_MAX_TOKENS":   "",
		"ANTHROPIC_API_KEY": "sk-test-fake",
		"XDG_DATA_HOME":     t.TempDir(),
	}
	maps.Copy(defaults, overrides)
	for k, v := range defaults {
		t.Setenv(k, v)
	}
}

// launchChat mirrors the production no-subcommand path: CobraDepsProvider →
// chat tool's Run. Tests use this so they exercise exactly the same chain
// `bite` walks at runtime, without going through cobra's parser.
func launchChat(ctx context.Context, resumeID int64) error {
	deps, cleanup, err := CobraDepsProvider(ctx)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	chat := MustGet("chat")
	_, err = chat.Run(ctx, deps, NewArgsForTool(chat, map[string]any{"resume": resumeID}))
	return err
}

// skipIfWindowsTTYHang skips tests that expect bubbletea to fail fast in a
// non-interactive environment. Linux/macOS CI runs without a terminal so
// prog.Run errors immediately, but Windows GitHub Actions presents a real
// console to test processes — prog.Run blocks for input until the test
// timeout (10 min). Production behavior is identical; only the test
// assumption is platform-dependent.
func skipIfWindowsTTYHang(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Windows CI presents a real console — bubbletea hangs instead of failing fast")
	}
}

func TestLaunchChat_configError(t *testing.T) {
	stubChatEnv(t, map[string]string{"BITE_MAX_TOKENS": "not-a-number"})
	require.Error(t, launchChat(context.Background(), 0))
}

func TestLaunchChat_openStoreError(t *testing.T) {
	// Pointing BITE_DB at a directory (not a file) makes db.Open fail.
	// t.TempDir() keeps this cross-platform — "/tmp" only works on unix.
	stubChatEnv(t, map[string]string{"BITE_DB": t.TempDir()})
	require.Error(t, launchChat(context.Background(), 0))
}

func TestLaunchChat_missingKey(t *testing.T) {
	stubChatEnv(t, map[string]string{"ANTHROPIC_API_KEY": ""})
	require.Error(t, launchChat(context.Background(), 0))
}

func TestLaunchChat_resumeNotFound(t *testing.T) {
	stubChatEnv(t, nil)
	require.Error(t, launchChat(context.Background(), 9999))
}

func TestLaunchChat_noTTY(t *testing.T) {
	skipIfWindowsTTYHang(t)
	stubChatEnv(t, nil)
	if launchChat(context.Background(), 0) == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
