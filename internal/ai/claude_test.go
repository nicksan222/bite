package ai

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClaudeModel_emptyKey(t *testing.T) {
	_, err := newClaudeModel(context.Background(), "", "claude-haiku-4-5", 0)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "missing API key")
}

func TestNewClaudeModel_emptyModel(t *testing.T) {
	_, err := newClaudeModel(context.Background(), "sk-test", "", 0)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "missing model id")
}

func TestNewClaudeModel_success(t *testing.T) {
	m, err := newClaudeModel(context.Background(), "sk-ant-test-fake", "claude-haiku-4-5", 1024)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}
