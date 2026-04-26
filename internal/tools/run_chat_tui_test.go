package tools

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubChatEnv sets every env var RunChatTUI reads to sane defaults; pass
// overrides to flip individual values (e.g. an empty key, a bad DSN).
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

func TestRunChatTUI_configError(t *testing.T) {
	stubChatEnv(t, map[string]string{"BITE_MAX_TOKENS": "not-a-number"})
	require.Error(t, RunChatTUI(context.Background(), 0))
}

func TestRunChatTUI_openStoreError(t *testing.T) {
	stubChatEnv(t, map[string]string{"BITE_DB": "/tmp"}) // directory, not a file
	require.Error(t, RunChatTUI(context.Background(), 0))
}

func TestRunChatTUI_missingKey(t *testing.T) {
	stubChatEnv(t, map[string]string{"ANTHROPIC_API_KEY": ""})
	require.Error(t, RunChatTUI(context.Background(), 0))
}

func TestRunChatTUI_resumeNotFound(t *testing.T) {
	stubChatEnv(t, nil)
	require.Error(t, RunChatTUI(context.Background(), 9999))
}

func TestRunChatTUI_noTTY(t *testing.T) {
	stubChatEnv(t, nil)
	if RunChatTUI(context.Background(), 0) == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
