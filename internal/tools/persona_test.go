package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildSystemPrompt_emptyUsesDefault(t *testing.T) {
	got := BuildSystemPrompt("")
	assert.True(t, strings.HasPrefix(got, DefaultPersona))
	assert.Contains(t, got, "## Available tools")
}

func TestBuildSystemPrompt_customOverride(t *testing.T) {
	got := BuildSystemPrompt("be terse")
	assert.True(t, strings.HasPrefix(got, "be terse"))
	assert.NotContains(t, got, DefaultPersona)
	assert.Contains(t, got, "## Available tools")
}

func TestBuildSystemPrompt_trimsTrailingNewlines(t *testing.T) {
	got := BuildSystemPrompt("custom\n\n")
	// Should not double-up newlines between custom and appendix.
	assert.NotContains(t, got, "custom\n\n\n\n##")
}
