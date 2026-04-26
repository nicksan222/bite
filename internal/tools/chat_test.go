package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatTool_isRegistered(t *testing.T) {
	tool, ok := Get("chat")
	require.True(t, ok, "chat must register itself like any other tool")
	assert.True(t, tool.SkipAI, "chat must opt out of the AI tool spec")
	assert.True(t, tool.SkipSlash, "chat must opt out of /chat dispatch")
}

func TestChatTool_resumeFlagSurfaces(t *testing.T) {
	tool := MustGet("chat")
	var found bool
	for _, p := range tool.Params {
		if p.Name == "resume" && p.Type == ParamInt {
			found = true
			break
		}
	}
	assert.True(t, found, "chat needs a --resume int flag for cobra binding")
}

func TestRunChat_propagatesPrepareSessionError(t *testing.T) {
	// resume=9999 against a fresh store can't be found; PrepareSession returns
	// an error which runChat must propagate so the cobra layer can surface it.
	deps := freshDeps(t)
	tool := MustGet("chat")
	_, err := tool.Run(context.Background(), deps, NewArgsForTool(tool, map[string]any{"resume": int64(9999)}))
	require.Error(t, err)
}

// TestRunChat_failsFastWhenAIUnusable proves the cobra path (lazyAI) errors
// before the TUI opens when ANTHROPIC_API_KEY is absent. Without
// Deps.RequireAI / lazyAI.EnsureUsable, the user would see the TUI launch
// then break on the first message — far worse UX than the historical eager
// fail-fast.
func TestRunChat_failsFastWhenAIUnusable(t *testing.T) {
	stubChatEnv(t, map[string]string{"ANTHROPIC_API_KEY": ""})
	deps, cleanup, err := CobraDepsProvider(context.Background())
	require.NoError(t, err)
	if cleanup != nil {
		defer cleanup()
	}
	tool := MustGet("chat")
	_, runErr := tool.Run(context.Background(), deps, NewArgsForTool(tool, nil))
	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "ANTHROPIC_API_KEY")
}
