package tools

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsk_returnsAccumulatedReply(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: "pong"}

	res, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{
		"prompt": "ping",
	}))
	require.NoError(t, err)
	assert.Equal(t, "pong", res.Text)
}

func TestAsk_writesDeltasToStreamWriter(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: "hello"}
	var buf bytes.Buffer
	deps.StreamWriter = &buf

	_, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{
		"prompt": "hi",
	}))
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "hello")
}

func TestAsk_noPromptOrImage(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: ""}
	_, err := MustGet("ask").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}

func TestAsk_noAIClient(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{
		"prompt": "hi",
	}))
	require.Error(t, err)
}

func TestAsImageAttachment_classifiesURLvsPath(t *testing.T) {
	got := asImageAttachment("https://example.com/x.jpg")
	assert.Equal(t, "https://example.com/x.jpg", got.URL)
	assert.Empty(t, got.Path)

	got = asImageAttachment("http://example.com/x.jpg")
	assert.Equal(t, "http://example.com/x.jpg", got.URL)

	got = asImageAttachment("/local/x.jpg")
	assert.Equal(t, "/local/x.jpg", got.Path)
	assert.Empty(t, got.URL)
}
