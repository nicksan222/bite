package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── ProviderSpec.Validate ────────────────────────────────────────────────────

func TestValidate_missingModel(t *testing.T) {
	s := ProviderSpec{Name: "x", DefaultModel: "d", CredentialEnv: "X_KEY"}
	err := s.Validate(ProviderConfig{APIKey: "k"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing model id")
}

func TestValidate_missingCredentialReturnsTypedError(t *testing.T) {
	s := ProviderSpec{Name: "x", DefaultModel: "d", CredentialEnv: "X_KEY"}
	err := s.Validate(ProviderConfig{Model: "m"})
	require.Error(t, err)
	var miss *MissingCredentialError
	require.ErrorAs(t, err, &miss)
	assert.Equal(t, "X_KEY", miss.EnvVar)
	assert.Equal(t, Provider("x"), miss.Provider)
	// The error message names the env var so consumers don't have to
	// build it themselves.
	assert.Contains(t, err.Error(), "X_KEY")
}

func TestValidate_credentialFreeProviderPasses(t *testing.T) {
	s := ProviderSpec{Name: "x", DefaultModel: "d"}
	err := s.Validate(ProviderConfig{Model: "m"})
	require.NoError(t, err)
}

// ─── ProviderSpec.LoadConfig ──────────────────────────────────────────────────

func TestLoadConfig_readsEnvForCredentialAndBaseURL(t *testing.T) {
	t.Setenv("X_KEY", "secret")
	t.Setenv("X_URL", "http://example")
	s := ProviderSpec{Name: "x", DefaultModel: "d-default", CredentialEnv: "X_KEY", BaseURLEnv: "X_URL", DefaultBaseURL: "http://default"}
	got := s.LoadConfig("", 0)
	assert.Equal(t, "secret", got.APIKey)
	assert.Equal(t, "http://example", got.BaseURL)
	assert.Equal(t, "d-default", got.Model)
}

func TestLoadConfig_modelOverrideWinsOverDefault(t *testing.T) {
	s := ProviderSpec{Name: "x", DefaultModel: "d-default"}
	got := s.LoadConfig("user-pick", 0)
	assert.Equal(t, "user-pick", got.Model)
}

func TestLoadConfig_baseURLFallsBackToDefault(t *testing.T) {
	t.Setenv("X_URL", "")
	s := ProviderSpec{Name: "x", DefaultModel: "d", BaseURLEnv: "X_URL", DefaultBaseURL: "http://default"}
	got := s.LoadConfig("", 0)
	assert.Equal(t, "http://default", got.BaseURL)
}

// ─── Registry ────────────────────────────────────────────────────────────────

func TestRegisterProvider_duplicatePanics(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()

	build := func(_ context.Context, _ ProviderConfig) (model.ToolCallingChatModel, error) { return nil, nil }
	RegisterProvider(ProviderSpec{Name: "dup", DefaultModel: "m", Build: build})

	assert.Panics(t, func() {
		RegisterProvider(ProviderSpec{Name: "dup", DefaultModel: "m", Build: build})
	})
}

func TestRegisterProvider_missingNamePanics(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()

	build := func(_ context.Context, _ ProviderConfig) (model.ToolCallingChatModel, error) { return nil, nil }
	assert.Panics(t, func() {
		RegisterProvider(ProviderSpec{DefaultModel: "m", Build: build})
	})
}

func TestRegisterProvider_missingBuildPanics(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()

	assert.Panics(t, func() {
		RegisterProvider(ProviderSpec{Name: "x", DefaultModel: "m"})
	})
}

func TestProviders_orderedByPriorityThenName(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()

	build := func(_ context.Context, _ ProviderConfig) (model.ToolCallingChatModel, error) { return nil, nil }
	RegisterProvider(ProviderSpec{Name: "b", DefaultModel: "m", Priority: 20, Build: build})
	RegisterProvider(ProviderSpec{Name: "a", DefaultModel: "m", Priority: 20, Build: build})
	RegisterProvider(ProviderSpec{Name: "z", DefaultModel: "m", Priority: 10, Build: build})

	got := Providers()
	require.Len(t, got, 3)
	assert.Equal(t, Provider("z"), got[0].Name, "lower priority first")
	assert.Equal(t, Provider("a"), got[1].Name, "alphabetical within same priority")
	assert.Equal(t, Provider("b"), got[2].Name)
}

// ─── Resolve ─────────────────────────────────────────────────────────────────

func TestResolve_explicitWinsEvenWhenUnconfigured(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()
	registerStubProviders(t)
	t.Setenv("A_KEY", "")
	t.Setenv("B_KEY", "set")

	spec, err := Resolve(Provider("a"))
	require.NoError(t, err)
	assert.Equal(t, Provider("a"), spec.Name)
}

func TestResolve_explicitUnknownErrors(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()
	registerStubProviders(t)

	_, err := Resolve(Provider("nope"))
	require.Error(t, err)
	assert.ErrorContains(t, err, `unknown provider "nope"`)
}

func TestResolve_picksFirstCredentialedProvider(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()
	registerStubProviders(t)
	t.Setenv("A_KEY", "")
	t.Setenv("B_KEY", "set")

	spec, err := Resolve("")
	require.NoError(t, err)
	assert.Equal(t, Provider("b"), spec.Name)
}

func TestResolve_fallsBackToFirstWhenNothingSet(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()
	registerStubProviders(t)
	t.Setenv("A_KEY", "")
	t.Setenv("B_KEY", "")

	spec, err := Resolve("")
	require.NoError(t, err)
	assert.Equal(t, Provider("a"), spec.Name, "fallback gives missing-key errors a sensible default")
}

func TestResolve_emptyRegistryErrors(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()

	_, err := Resolve("")
	require.Error(t, err)
	assert.ErrorContains(t, err, "no providers registered")
}

// ─── AnyConfigured ───────────────────────────────────────────────────────────

func TestAnyConfigured_passesWhenOneCredentialedProviderHasKey(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()
	registerStubProviders(t)
	t.Setenv("A_KEY", "")
	t.Setenv("B_KEY", "set")

	require.NoError(t, AnyConfigured())
}

func TestAnyConfigured_failsWhenNoneSet(t *testing.T) {
	defer restoreProviders(t)()
	resetProvidersForTest()
	registerStubProviders(t)
	t.Setenv("A_KEY", "")
	t.Setenv("B_KEY", "")

	err := AnyConfigured()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoProviderConfigured)
	assert.ErrorContains(t, err, "A_KEY")
	assert.ErrorContains(t, err, "B_KEY")
}

func TestAnyConfigured_skipsCredentialFreeProviders(t *testing.T) {
	// Ollama-style providers (no CredentialEnv) shouldn't satisfy the
	// "anything configured?" check on their own — they're opt-in.
	defer restoreProviders(t)()
	resetProvidersForTest()
	build := func(_ context.Context, _ ProviderConfig) (model.ToolCallingChatModel, error) { return nil, nil }
	RegisterProvider(ProviderSpec{Name: "local", DefaultModel: "m", Build: build})

	require.NoError(t, AnyConfigured(), "no credentialed providers means nothing to check — passes vacuously")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func registerStubProviders(t *testing.T) {
	t.Helper()
	build := func(_ context.Context, _ ProviderConfig) (model.ToolCallingChatModel, error) {
		return nil, errors.New("stub")
	}
	RegisterProvider(ProviderSpec{Name: "a", DefaultModel: "m", CredentialEnv: "A_KEY", Priority: 10, Build: build})
	RegisterProvider(ProviderSpec{Name: "b", DefaultModel: "m", CredentialEnv: "B_KEY", Priority: 20, Build: build})
}

// resetProvidersForTest clears the registry between table cases. Used
// only from tests that exercise registration semantics.
func resetProvidersForTest() {
	providersMu.Lock()
	defer providersMu.Unlock()
	providers = map[Provider]ProviderSpec{}
}

// restoreProviders snapshots the current registry and returns a cleanup
// that puts every real provider back. Tests that mutate the registry
// use it to keep the package-level init-registered set intact for
// siblings.
func restoreProviders(t *testing.T) func() {
	t.Helper()
	providersMu.RLock()
	snapshot := make([]ProviderSpec, 0, len(providers))
	for _, s := range providers {
		snapshot = append(snapshot, s)
	}
	providersMu.RUnlock()
	return func() {
		resetProvidersForTest()
		for _, s := range snapshot {
			RegisterProvider(s)
		}
	}
}
