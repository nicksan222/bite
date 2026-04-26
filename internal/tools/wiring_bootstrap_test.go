package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
)

// recordingHooks captures every hook call so tests can assert what the
// orchestration did without firing real installers, prompts, or DB
// writes.
type recordingHooks struct {
	consent        bool
	consentRecord  int
	promptResult   bool
	promptCalls    int
	bootstrapErr   error
	bootstrapCalls int
	bootstrapModel string
	bootstrapURL   string
}

func (r *recordingHooks) hooks() bootstrapHooks {
	return bootstrapHooks{
		HasConsent:    func(context.Context) bool { return r.consent },
		RecordConsent: func(context.Context) error { r.consentRecord++; return nil },
		PromptConsent: func() bool { r.promptCalls++; return r.promptResult },
		Bootstrap: func(_ context.Context, model, baseURL string) error {
			r.bootstrapCalls++
			r.bootstrapModel = model
			r.bootstrapURL = baseURL
			return r.bootstrapErr
		},
	}
}

func clearProviderEnv(t *testing.T) {
	t.Helper()
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("BITE_PROVIDER", "")
}

// ─── explicit Ollama selection ──────────────────────────────────────────────

func TestEnsureProviderOrBootstrap_explicitOllamaRunsBootstrapWithoutPrompt(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{} // PromptConsent left as no, HasConsent false
	cfg, err := ensureProviderOrBootstrap(context.Background(),
		config.Config{Provider: string(ai.ProviderOllama)}, r.hooks())
	require.NoError(t, err)
	assert.Equal(t, string(ai.ProviderOllama), cfg.Provider)
	assert.Zero(t, r.promptCalls, "explicit selection skips the prompt entirely")
	assert.Equal(t, 1, r.bootstrapCalls, "still bootstraps to ensure daemon + model are ready")
	assert.Zero(t, r.consentRecord, "explicit selection doesn't need persisted consent")
}

func TestEnsureProviderOrBootstrap_explicitOllamaSurfacesBootstrapErrors(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{bootstrapErr: errors.New("ollama not on PATH")}
	_, err := ensureProviderOrBootstrap(context.Background(),
		config.Config{Provider: string(ai.ProviderOllama)}, r.hooks())
	require.Error(t, err)
	assert.ErrorContains(t, err, "ollama not on PATH")
}

func TestEnsureProviderOrBootstrap_passesUserModelAndURLToBootstrap(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OLLAMA_BASE_URL", "http://gpu-box:11434")
	r := &recordingHooks{}
	_, err := ensureProviderOrBootstrap(context.Background(),
		config.Config{Provider: string(ai.ProviderOllama), Model: "qwen3:14b"},
		r.hooks())
	require.NoError(t, err)
	assert.Equal(t, "qwen3:14b", r.bootstrapModel,
		"BITE_MODEL must reach the bootstrap so we don't pull the wrong tag")
	assert.Equal(t, "http://gpu-box:11434", r.bootstrapURL,
		"OLLAMA_BASE_URL must reach the bootstrap so we don't probe the wrong host")
}

// ─── happy paths ─────────────────────────────────────────────────────────────

func TestEnsureProviderOrBootstrap_passesThroughWhenProviderHasCredentials(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	r := &recordingHooks{}
	cfg, err := ensureProviderOrBootstrap(context.Background(),
		config.Config{Provider: "anthropic"}, r.hooks())
	require.NoError(t, err)
	assert.Equal(t, "anthropic", cfg.Provider)
	assert.Zero(t, r.promptCalls, "no prompt — credentials are present")
	assert.Zero(t, r.bootstrapCalls, "no bootstrap — credentials are present")
}

func TestEnsureProviderOrBootstrap_passesThroughWhenAnyProviderConfigured(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("OPENAI_API_KEY", "sk-test")
	r := &recordingHooks{}
	cfg, err := ensureProviderOrBootstrap(context.Background(),
		config.Config{}, r.hooks()) // empty provider → auto-resolve to openai
	require.NoError(t, err)
	assert.Empty(t, cfg.Provider, "auto-resolution happens later in BuildAIClient")
	assert.Zero(t, r.promptCalls)
	assert.Zero(t, r.bootstrapCalls)
}

// ─── consent prompt branch ──────────────────────────────────────────────────

func TestEnsureProviderOrBootstrap_promptsAndBootstrapsOnYes(t *testing.T) {
	clearProviderEnv(t)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	r := &recordingHooks{promptResult: true}

	cfg, err := ensureProviderOrBootstrap(context.Background(), config.Config{}, r.hooks())
	require.NoError(t, err)
	assert.Equal(t, string(ai.ProviderOllama), cfg.Provider, "switch to Ollama after successful bootstrap")
	assert.Equal(t, 1, r.promptCalls)
	assert.Equal(t, 1, r.bootstrapCalls)
	assert.Equal(t, 1, r.consentRecord, "consent persisted after bootstrap")
}

func TestEnsureProviderOrBootstrap_promptDeclinedReturnsRegistryError(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{promptResult: false}

	_, err := ensureProviderOrBootstrap(context.Background(), config.Config{}, r.hooks())
	require.Error(t, err)
	assert.ErrorIs(t, err, ai.ErrNoProviderConfigured, "user said no, surface the registry's standard error")
	assert.Equal(t, 1, r.promptCalls)
	assert.Zero(t, r.bootstrapCalls)
	assert.Zero(t, r.consentRecord)
}

func TestEnsureProviderOrBootstrap_bootstrapFailureSurfaced(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{promptResult: true, bootstrapErr: errors.New("disk full")}

	_, err := ensureProviderOrBootstrap(context.Background(), config.Config{}, r.hooks())
	require.Error(t, err)
	assert.ErrorContains(t, err, "disk full")
	assert.Zero(t, r.consentRecord, "bootstrap failed → don't persist consent")
}

// ─── persisted consent skips the prompt ─────────────────────────────────────

func TestEnsureProviderOrBootstrap_skipsPromptWhenConsentRecorded(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{consent: true}

	cfg, err := ensureProviderOrBootstrap(context.Background(), config.Config{}, r.hooks())
	require.NoError(t, err)
	assert.Equal(t, string(ai.ProviderOllama), cfg.Provider)
	assert.Zero(t, r.promptCalls, "stored consent means no re-prompt")
	assert.Equal(t, 1, r.bootstrapCalls, "still re-runs bootstrap (idempotent)")
	assert.Zero(t, r.consentRecord, "no need to re-record")
}

func TestEnsureProviderOrBootstrap_consentedBootstrapErrorSurfaces(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{consent: true, bootstrapErr: errors.New("ollama daemon unreachable")}

	_, err := ensureProviderOrBootstrap(context.Background(), config.Config{}, r.hooks())
	require.Error(t, err)
	assert.ErrorContains(t, err, "ollama daemon unreachable")
}

// ─── ResolvedModel ───────────────────────────────────────────────────────────

func TestLazyAI_ResolvedModelReflectsBootstrapSwitch(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{consent: true} // skip prompt, run bootstrap
	la := &lazyAI{cfg: config.Config{}, hooks: r.hooks()}

	// Before ensure runs the model is the auto-resolved fallback. After
	// ensure flips Provider to ollama, ResolvedModel must return the
	// Ollama spec's default — not the startup-time anthropic default.
	gotModel := la.ResolvedModel()
	ollamaSpec, _ := ai.Lookup(ai.ProviderOllama)
	assert.Equal(t, ollamaSpec.DefaultModel, gotModel,
		"after consented bootstrap, model column must record what actually answered")
}

// ─── once-per-process ───────────────────────────────────────────────────────

func TestLazyAI_ensureRunsBootstrapOnlyOnce(t *testing.T) {
	clearProviderEnv(t)
	r := &recordingHooks{consent: true}

	la := &lazyAI{cfg: config.Config{}, hooks: r.hooks()}
	_, _ = la.ensure(context.Background())
	_, _ = la.ensure(context.Background())
	_, _ = la.ensure(context.Background())

	assert.Equal(t, 1, r.bootstrapCalls, "sync.Once must coalesce repeated calls")
}
