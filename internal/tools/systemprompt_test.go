package tools

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderAppendix_emptyRegistry(t *testing.T) {
	withCleanRegistry(t, func() {
		assert.Empty(t, RenderAppendix())
	})
}

func TestRenderAppendix_listsEveryTool(t *testing.T) {
	out := RenderAppendix()
	assert.Contains(t, out, "## Available tools")
	for _, tool := range All() {
		if tool.SkipAI {
			// Cobra-only tools (chat) are deliberately omitted — the model
			// can't usefully call them.
			continue
		}
		assert.Contains(t, out, tool.Name, "appendix should mention %q", tool.Name)
		assert.Contains(t, out, tool.Summary, "appendix should mention %q summary", tool.Name)
	}
}

func TestRenderAppendix_alphabetical(t *testing.T) {
	out := RenderAppendix()
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "### ") {
			names = append(names, strings.TrimPrefix(line, "### "))
		}
	}
	require := assert.New(t)
	for i := 1; i < len(names); i++ {
		require.LessOrEqual(names[i-1], names[i], "tool list not sorted: %v", names)
	}
}

func TestRenderAppendix_promptOverridesDescription(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name:        "guarded",
			Summary:     "s",
			Description: "long description",
			Prompt:      "CALL THIS WHEN X HAPPENS",
			Run:         noopTool("x").Run,
		})
		out := RenderAppendix()
		assert.Contains(t, out, "CALL THIS WHEN X HAPPENS")
		assert.NotContains(t, out, "long description")
	})
}
