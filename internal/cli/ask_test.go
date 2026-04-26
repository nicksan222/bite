package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

func TestImageAttachment_url(t *testing.T) {
	cases := []struct {
		input string
		url   string
		path  string
	}{
		{"https://example.com/food.jpg", "https://example.com/food.jpg", ""},
		{"http://example.com/meal.png", "http://example.com/meal.png", ""},
		{"/local/path/img.jpg", "", "/local/path/img.jpg"},
		{"relative/path.jpg", "", "relative/path.jpg"},
	}
	for _, c := range cases {
		got := imageAttachment(c.input)
		assert.Equal(t, c.url, got.URL, "input %q: URL", c.input)
		assert.Equal(t, c.path, got.Path, "input %q: Path", c.input)
	}
}

func TestRunAsk_missingKey(t *testing.T) {
	// runAsk with missing API key → openAIClient fails early.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("system", "s", "", "system")
	cmd.Flags().StringSliceP("image", "i", nil, "images")

	err := runAsk(cmd, []string{"is pasta healthy?"})
	require.Error(t, err)
}

func TestImageAttachment_returnsAttachment(t *testing.T) {
	a := imageAttachment("meal.jpg")
	// should be an Attachment (just verify it's the right type)
	_ = ai.Message{Attachments: []ai.Attachment{a}}
}

// mockStreamer is a reusable mock for Streamer in ask/analyze tests.
type mockStreamer struct {
	events []ai.StreamEvent
	err    error // error returned from Stream itself
}

func (s *mockStreamer) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan ai.StreamEvent, len(s.events))
	for _, ev := range s.events {
		ch <- ev
	}
	close(ch)
	return ch, nil
}

func TestStreamAsk_success(t *testing.T) {
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: "hello"},
		{Delta: " world"},
		{Done: true, Final: "hello world"},
	}}
	var buf bytes.Buffer
	err := streamAsk(context.Background(), &buf, client, "hi", nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "hello")
}

func TestStreamAsk_streamError(t *testing.T) {
	client := &mockStreamer{err: errors.New("stream open failed")}
	err := streamAsk(context.Background(), &bytes.Buffer{}, client, "hi", nil)
	require.Error(t, err)
}

func TestStreamAsk_eventError(t *testing.T) {
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: "partial"},
		{Err: errors.New("mid-stream error")},
	}}
	err := streamAsk(context.Background(), &bytes.Buffer{}, client, "hi", nil)
	require.Error(t, err)
}

func TestStreamAsk_withImages(t *testing.T) {
	client := &mockStreamer{events: []ai.StreamEvent{
		{Done: true, Final: ""},
	}}
	var buf bytes.Buffer
	err := streamAsk(context.Background(), &buf, client, "describe this", []string{"meal.jpg", "https://example.com/img.png"})
	require.NoError(t, err)
}

func TestStreamAsk_channelClosesWithoutDone(t *testing.T) {
	// Channel closes without a Done event → last return nil is reached.
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: "partial output"},
		// no Done event — channel just closes
	}}
	var buf bytes.Buffer
	err := streamAsk(context.Background(), &buf, client, "hi", nil)
	require.NoError(t, err)
}

func TestRunAsk_stdinReadError(t *testing.T) {
	// Replace os.Stdin with a closed file so io.ReadAll returns an error.
	r, _, err := os.Pipe()
	require.NoError(t, err)
	r.Close() // close read end → Read returns error

	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("system", "s", "", "system")
	cmd.Flags().StringSliceP("image", "i", nil, "images")

	askErr := runAsk(cmd, nil)
	require.Error(t, askErr)
}

func TestRunAsk_systemPromptOverride(t *testing.T) {
	// sys flag set → covers cfg.SystemPrompt = sys branch.
	// Then openAIClient fails (no API key), covering line 64.
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("system", "s", "", "system")
	cmd.Flags().StringSliceP("image", "i", nil, "images")
	require.NoError(t, cmd.Flags().Set("system", "be brief"))

	err := runAsk(cmd, []string{"is pasta healthy?"})
	require.Error(t, err)
}

func TestRunAsk_cancelledContext(t *testing.T) {
	// Pre-cancelled context → passes all setup, reaches streamAsk which fails.
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	cmd.Flags().StringP("system", "s", "", "system")
	cmd.Flags().StringSliceP("image", "i", nil, "images")

	err := runAsk(cmd, []string{"is pasta healthy?"})
	require.Error(t, err)
}

func TestRunAsk_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("system", "s", "", "system")
	cmd.Flags().StringSliceP("image", "i", nil, "images")

	err := runAsk(cmd, []string{"is pasta healthy?"})
	require.Error(t, err)
}

func TestRunAsk_emptyPromptNoImages(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("system", "s", "", "system")
	cmd.Flags().StringSliceP("image", "i", nil, "images")

	// no args and no --image → "nothing to send" error
	err := runAsk(cmd, nil)
	require.Error(t, err)
}
