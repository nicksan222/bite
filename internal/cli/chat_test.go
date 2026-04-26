package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunChat_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	require.Error(t, err)
}

func TestRunChat_openStoreError(t *testing.T) {
	t.Setenv("BITE_DB", "/tmp") // directory, not a file
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	require.Error(t, err)
}

func TestRunChat_missingKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	require.Error(t, err)
}

func TestRunChat_resumeNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 9999)
	require.Error(t, err)
}

func TestRunChat_noTTY(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	if err == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
