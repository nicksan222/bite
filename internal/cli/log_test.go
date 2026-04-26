package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

// mockStreamerForLog returns a fixed MealAnalysis JSON.
type mockStreamerForLog struct {
	resp string
	err  error
}

func (s *mockStreamerForLog) Stream(_ context.Context, _ []ai.Message, _ ...ai.StreamOption) (<-chan ai.StreamEvent, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan ai.StreamEvent, 2)
	ch <- ai.StreamEvent{Delta: s.resp}
	ch <- ai.StreamEvent{Done: true, Final: s.resp}
	close(ch)
	return ch, nil
}

const logMealJSON = `{"title":"pasta","description":"pasta with pesto","items":["pasta","pesto"],"kcal":550,"protein_g":18,"carbs_g":80,"fat_g":12,"confidence":"high"}`

func TestLogMeal_savesAndPrints(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	client := &mockStreamerForLog{resp: logMealJSON}

	var buf bytes.Buffer
	require.NoError(t, logMeal(ctx, &buf, client, store, "200g pasta with pesto", nil, ""))

	out := buf.String()
	assert.Contains(t, out, "pasta")
	assert.Contains(t, out, "550")

	// Verify meal was saved to store
	meals, _ := store.ListRecentMeals(ctx, 10)
	require.Len(t, meals, 1)
	assert.Equal(t, "pasta", meals[0].Title)
}

func TestLogMeal_withURLFiles(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	client := &mockStreamerForLog{resp: logMealJSON}

	var buf bytes.Buffer
	files := []string{"https://example.com/meal.jpg", "/local/path.jpg"}
	require.NoError(t, logMeal(ctx, &buf, client, store, "my meal", files, ""))
}

func TestLogMeal_analyzeError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	client := &mockStreamerForLog{err: errors.New("api down")}

	err := logMeal(ctx, &bytes.Buffer{}, client, store, "pasta", nil, "")
	require.Error(t, err)
}

func TestRunLog_analyzeError(t *testing.T) {
	// runLog with a real store but analysis fails (no API key).
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	// No ANTHROPIC_API_KEY → openAIClient fails → runLog returns error early.

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringArrayP("file", "f", nil, "files")
	cmd.SetOut(nil)

	err := runLog(cmd, []string{"pasta"})
	require.Error(t, err)
}

func TestRunLog_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringArrayP("file", "f", nil, "files")

	err := runLog(cmd, []string{"pasta"})
	require.Error(t, err)
}

func TestRunLog_reachesLogMeal(t *testing.T) {
	// Valid config + valid store → covers defer/files/logMeal lines in runLog.
	// logMeal fails early (audio without OPENAI_API_KEY) — no network call.
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringArrayP("file", "f", nil, "files")
	require.NoError(t, cmd.Flags().Set("file", "/fake/meal.mp3"))
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runLog(cmd, []string{"had a meal"})
	// audio file + no OPENAI_API_KEY → preprocess returns error immediately
	require.Error(t, err)
}

func TestRunLog_openStoreError(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("BITE_DB", "/tmp") // directory, not a file → store open fails
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringArrayP("file", "f", nil, "files")

	err := runLog(cmd, []string{"pasta"})
	require.Error(t, err)
}

func TestLogMeal_saveError(t *testing.T) {
	ctx := context.Background()
	// Return analysis JSON with empty title — SaveMeal requires non-empty title.
	badJSON := `{"title":"","kcal":100}`
	client := &mockStreamerForLog{resp: badJSON}
	store := newMockStore()

	err := logMeal(ctx, &bytes.Buffer{}, client, store, "something", nil, "")
	require.Error(t, err)
}
