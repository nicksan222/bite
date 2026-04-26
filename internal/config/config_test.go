package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("BITE_DB", "")
	t.Setenv("BITE_PROVIDER", "")
	t.Setenv("BITE_MODEL", "")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("BITE_SYSTEM_PROMPT", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	c, err := Load()
	require.NoError(t, err)

	assert.Empty(t, c.Provider, "provider stays empty so the AI registry resolves it")
	assert.Empty(t, c.Model, "model stays empty so each provider falls back to its DefaultModel")
	assert.Equal(t, 4096, c.MaxTokens)
	assert.Equal(t, "bite.db", filepath.Base(c.DSN))
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "custom.db")
	t.Setenv("BITE_DB", dsn)
	t.Setenv("BITE_PROVIDER", "openai")
	t.Setenv("BITE_MODEL", "gpt-4o")
	t.Setenv("BITE_MAX_TOKENS", "1024")
	t.Setenv("BITE_SYSTEM_PROMPT", "be terse")
	t.Setenv("OPENAI_API_KEY", "sk-test")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, dsn, c.DSN)
	assert.Equal(t, "openai", c.Provider)
	assert.Equal(t, "gpt-4o", c.Model)
	assert.Equal(t, 1024, c.MaxTokens)
	assert.Equal(t, "be terse", c.SystemPrompt)
	assert.Equal(t, "sk-test", c.OpenAIAPIKey)
}

func TestInvalidMaxTokens(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "0")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	_, err := Load()
	require.Error(t, err)
}

func TestLoad_customDSNCreatesDir(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "subdir", "custom.db")
	t.Setenv("BITE_DB", dsn)
	t.Setenv("BITE_MAX_TOKENS", "")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, dsn, c.DSN)
}

func TestLoad_invalidMaxTokensEnvVar(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	_, err := Load()
	require.Error(t, err)
}

func TestLoad_uncreatableDataDir(t *testing.T) {
	// /dev/null is a char device — MkdirAll of /dev/null/sub will fail.
	// Windows has no equivalent guaranteed-uncreatable path under the user's
	// rights, so skip there; the unix branch alone is sufficient coverage.
	if runtime.GOOS == "windows" {
		t.Skip("no equivalent uncreatable path on Windows")
	}
	t.Setenv("BITE_DB", "/dev/null/sub/bite.db")
	t.Setenv("BITE_MAX_TOKENS", "")
	_, err := Load()
	require.Error(t, err)
}

func TestLoad_emptySystemPromptStaysEmpty(t *testing.T) {
	// The default persona lives in internal/tools (DefaultPersona); config
	// must not silently inject one so callers see one source of truth.
	t.Setenv("BITE_DB", t.TempDir()+"/bite.db")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("BITE_SYSTEM_PROMPT", "")

	c, err := Load()
	require.NoError(t, err)
	assert.Empty(t, c.SystemPrompt, "config should not fabricate a default; tools.BuildSystemPrompt does it")
}

func TestLoad_explicitSystemPromptPreserved(t *testing.T) {
	t.Setenv("BITE_DB", t.TempDir()+"/bite.db")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("BITE_SYSTEM_PROMPT", "custom prompt")

	c, err := Load()
	require.NoError(t, err)
	assert.Equal(t, "custom prompt", c.SystemPrompt)
}

func TestLoad_xdgDataFileError(t *testing.T) {
	t.Setenv("BITE_DB", "")
	t.Setenv("BITE_MAX_TOKENS", "")
	// Point XDG_DATA_HOME at a file (not a dir) so xdg.DataFile can't create subdirectories.
	f, err := os.CreateTemp("", "xdg-file-*")
	require.NoError(t, err)
	f.Close()
	defer os.Remove(f.Name())
	t.Setenv("XDG_DATA_HOME", f.Name())

	_, err = Load()
	if err == nil {
		t.Skip("xdg.DataFile did not fail with file as XDG_DATA_HOME — skip on this platform")
	}
}
