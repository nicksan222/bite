package tools

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
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

// errStreamer surfaces an error from Stream() before any events fire.
type errStreamer struct{ err error }

func (s errStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	return nil, s.err
}

// eventStreamer emits the supplied events in order. Used to drive runAsk's
// error and Done-only paths deterministically.
type eventStreamer struct{ events []ai.StreamEvent }

func (s eventStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	ch := make(chan ai.StreamEvent, len(s.events))
	for _, ev := range s.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestAsk_streamCallError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = errStreamer{err: errors.New("model down")}
	_, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{"prompt": "hi"}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "model down")
}

func TestAsk_streamEventError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = eventStreamer{events: []ai.StreamEvent{{Err: errors.New("boom")}}}
	_, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{"prompt": "hi"}))
	require.Error(t, err)
}

func TestAsk_finalOnlyWhenNoDeltas(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	// No Delta events, only Done with Final — runAsk should still return Final.
	deps.AI = eventStreamer{events: []ai.StreamEvent{{Done: true, Final: "answer"}}}
	res, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{"prompt": "hi"}))
	require.NoError(t, err)
	assert.Equal(t, "answer", res.Text)
}

func TestAsk_attachesImages(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: "ok"}

	res, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{
		"image": []any{"https://example.com/x.jpg", "/local/y.jpg"},
	}))
	require.NoError(t, err)
	assert.Equal(t, "ok", res.Text)
}

func TestAsk_channelClosedWithoutDone(t *testing.T) {
	// Some streamers may close the channel after deltas without ever emitting
	// a Done event. runAsk should still return what it accumulated.
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = eventStreamer{events: []ai.StreamEvent{{Delta: "partial"}}}
	res, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{"prompt": "hi"}))
	require.NoError(t, err)
	assert.Equal(t, "partial", res.Text)
}

func TestAsk_systemOverride(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	deps.AI = stubAI{resp: "ok"}
	_, err := MustGet("ask").Run(ctx, deps, NewArgs(map[string]any{
		"prompt": "hi",
		"system": "be terse",
	}))
	require.NoError(t, err)
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
