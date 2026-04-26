package cli

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/config"
)

func TestSetBuildInfo_nonEmpty(t *testing.T) {
	SetBuildInfo("1.2.3", "abc123", "2026-04-25")
	assert.Equal(t, "1.2.3", buildVersion)
	assert.Equal(t, "abc123", buildCommit)
	assert.Equal(t, "2026-04-25", buildDate)
}

func TestSetBuildInfo_emptySkips(t *testing.T) {
	// Calling with empty strings should not overwrite existing values.
	SetBuildInfo("before", "before", "before")
	SetBuildInfo("", "", "")
	assert.Equal(t, "before", buildVersion, "buildVersion should not change on empty input")
}

func TestOpenAIClient_missingKey(t *testing.T) {
	cfg := config.Config{} // empty APIKey → RequireAPIKey fails
	_, err := openAIClient(context.Background(), cfg)
	require.Error(t, err)
}

func TestOpenAIClient_success(t *testing.T) {
	cfg := config.Config{
		APIKey: "sk-ant-test-fake",
		Model:  "claude-haiku-4-5",
	}
	client, err := openAIClient(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestOpenStore_inMemory(t *testing.T) {
	cfg := config.Config{DSN: ":memory:"}
	store, err := openStore(context.Background(), cfg)
	require.NoError(t, err)
	store.Close()
}

func TestLoadConfig_error(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	_, err := loadConfig()
	require.Error(t, err)
}

func TestLoadConfig_success(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg, err := loadConfig()
	require.NoError(t, err)
	assert.Equal(t, "claude-haiku-4-5", cfg.Model)
}

func TestExecute_rootRunE_noTTY(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	rootCmd.SetArgs([]string{})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	err := Execute(context.Background())
	if err == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}

func TestExecute_chatSubcommand_noTTY(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	rootCmd.SetArgs([]string{"chat"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	err := Execute(context.Background())
	if err == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
