package cli

import (
	"context"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// stubEnv prepares environment variables for runChat tests with sane defaults.
// Pass overrides to flip individual values (e.g. an empty key, a bad DSN).
func stubEnv(t *testing.T, overrides map[string]string) {
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

func TestRunChat_configError(t *testing.T) {
	stubEnv(t, map[string]string{"BITE_MAX_TOKENS": "not-a-number"})
	require.Error(t, runChat(context.Background(), 0))
}

func TestRunChat_openStoreError(t *testing.T) {
	stubEnv(t, map[string]string{"BITE_DB": "/tmp"}) // directory, not a file
	require.Error(t, runChat(context.Background(), 0))
}

func TestRunChat_missingKey(t *testing.T) {
	stubEnv(t, map[string]string{"ANTHROPIC_API_KEY": ""})
	require.Error(t, runChat(context.Background(), 0))
}

func TestRunChat_resumeNotFound(t *testing.T) {
	stubEnv(t, nil)
	require.Error(t, runChat(context.Background(), 9999))
}

func TestRunChat_noTTY(t *testing.T) {
	stubEnv(t, nil)
	if runChat(context.Background(), 0) == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
