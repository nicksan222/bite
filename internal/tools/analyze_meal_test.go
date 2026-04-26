package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const analysisJSON = `{"title":"pasta","description":"with pesto","items":["pasta","pesto"],"kcal":550,"protein_g":18,"carbs_g":80,"fat_g":12,"confidence":"high"}`

func TestAnalyzeMeal_returnsAnalysis(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: analysisJSON}

	res, err := MustGet("analyze_meal").Run(ctx, deps, NewArgs(map[string]any{
		"text": "200g pasta",
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "pasta")
	assert.Contains(t, res.Text, "550")
	assert.Contains(t, res.Text, "confidence: high")
}

func TestAnalyzeMeal_emptyInput(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: analysisJSON}

	_, err := MustGet("analyze_meal").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}

func TestAnalyzeMeal_noAIClient(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	// deps.AI left nil
	_, err := MustGet("analyze_meal").Run(ctx, deps, NewArgs(map[string]any{
		"text": "x",
	}))
	require.Error(t, err)
}

func TestAnalyzeMeal_aiStreamError(t *testing.T) {
	// When the underlying AnalyzeMeal call errors (model returns bad JSON,
	// network blip, etc.), analyzeFromArgs wraps it. Drive the failure with
	// a stub that returns non-JSON.
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: "not json at all"}

	_, err := MustGet("analyze_meal").Run(ctx, deps, NewArgs(map[string]any{
		"text": "x",
	}))
	require.Error(t, err)
}

func TestAnalyzeMeal_routesURLsAndPaths(t *testing.T) {
	// analyzeFromArgs splits the file list into MediaURLs (http/https) vs
	// MediaPaths (everything else). Verify both branches are reachable.
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: analysisJSON}

	res, err := MustGet("analyze_meal").Run(ctx, deps, NewArgs(map[string]any{
		"text": "lunch",
		"file": []any{
			"https://example.com/photo.jpg",
			"http://example.com/photo2.jpg",
			"/local/photo.jpg",
		},
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "pasta")
}
