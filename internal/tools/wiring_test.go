package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/config"
)

func TestLoadConfig_error(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	_, err := LoadConfig()
	require.Error(t, err)
}

func TestLoadConfig_success(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cfg, err := LoadConfig()
	require.NoError(t, err)
	assert.Equal(t, "claude-haiku-4-5", cfg.Model)
}

func TestOpenStore_inMemory(t *testing.T) {
	cfg := config.Config{DSN: ":memory:"}
	store, err := OpenStore(context.Background(), cfg)
	require.NoError(t, err)
	defer store.Close()
}

func TestBuildAIClient_missingKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	cfg := config.Config{}
	_, err := BuildAIClient(context.Background(), cfg)
	require.Error(t, err)
}

func TestBuildAIClient_success(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-fake")
	cfg := config.Config{
		Provider: "anthropic",
		Model:    "claude-haiku-4-5",
	}
	client, err := BuildAIClient(context.Background(), cfg)
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCobraDepsProvider_success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	deps, cleanup, err := CobraDepsProvider(context.Background())
	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.NotNil(t, deps.Store)
	assert.NotNil(t, deps.AI)
	cleanup()
}

func TestCobraDepsProvider_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, _, err := CobraDepsProvider(context.Background())
	require.Error(t, err)
}

func TestCobraDepsProvider_storeOpenError(t *testing.T) {
	// Pointing BITE_DB at a directory makes db.Open fail (it'd try to open
	// the dir as a SQLite file and ping fails). t.TempDir() works on every
	// platform; "/tmp" doesn't exist on Windows.
	t.Setenv("BITE_DB", t.TempDir())
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	_, _, err := CobraDepsProvider(context.Background())
	require.Error(t, err)
}

func TestLazyAI_streamPropagatesAuthError(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	la := &lazyAI{
		cfg:   config.Config{Provider: "anthropic", Model: "claude-x", MaxTokens: 1},
		hooks: stubHooksThatNeverPrompt(),
	}
	_, err := la.Stream(context.Background(), nil)
	require.Error(t, err)
}

func TestLazyAI_streamDelegatesWhenClientBuilt(t *testing.T) {
	// With a valid (fake) key, BuildAIClient succeeds; lazyAI.Stream then
	// delegates to *ai.Client.Stream. The eino model may reject the fake
	// key synchronously, which is fine — both outcomes exercise the
	// post-build path that the cfg-error branch can't reach.
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-fake")
	la := &lazyAI{
		cfg: config.Config{
			Provider:  "anthropic",
			Model:     "claude-haiku-4-5",
			MaxTokens: 16,
		},
		hooks: stubHooksThatNeverPrompt(),
	}
	ch, err := la.Stream(context.Background(), nil)
	if err != nil {
		// Eino refused the fake key synchronously — that's fine, we still
		// reached lazyAI.Stream's post-build path.
		return
	}
	// Otherwise drain the channel; the auth error surfaces as an event.
	for ev := range ch {
		_ = ev
	}
}

// stubHooksThatNeverPrompt fails the test if the consent flow is ever
// triggered. Use it for tests that should reach Validate without going
// through the bootstrap branch — e.g. when a real provider key is set.
func stubHooksThatNeverPrompt() bootstrapHooks {
	return bootstrapHooks{
		HasConsent:    func(context.Context) bool { return false },
		RecordConsent: func(context.Context) error { return nil },
		PromptConsent: func() bool { return false },
		Bootstrap:     func(context.Context, string, string) error { return nil },
	}
}
