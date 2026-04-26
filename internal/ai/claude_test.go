package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeProviderRegistered(t *testing.T) {
	spec, ok := Lookup(ProviderAnthropic)
	require.True(t, ok)
	assert.Equal(t, "ANTHROPIC_API_KEY", spec.CredentialEnv)
	assert.NotEmpty(t, spec.DefaultModel)
}

func TestBuildClaudeModel_emptyKey(t *testing.T) {
	spec, _ := Lookup(ProviderAnthropic)
	err := spec.Validate(ProviderConfig{Model: "claude-haiku-4-5"})
	require.Error(t, err)
	var miss *MissingCredentialError
	require.ErrorAs(t, err, &miss)
	assert.Equal(t, "ANTHROPIC_API_KEY", miss.EnvVar)
}

func TestBuildClaudeModel_emptyModel(t *testing.T) {
	spec, _ := Lookup(ProviderAnthropic)
	err := spec.Validate(ProviderConfig{APIKey: "sk-ant-test"})
	require.Error(t, err)
	assert.ErrorContains(t, err, "missing model id")
}

func TestBuildClaudeModel_success(t *testing.T) {
	spec, _ := Lookup(ProviderAnthropic)
	m, err := spec.Build(context.Background(), ProviderConfig{
		APIKey:    "sk-ant-test-fake",
		Model:     "claude-haiku-4-5",
		MaxTokens: 1024,
	})
	require.NoError(t, err)
	assert.NotNil(t, m)
}
