package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

func TestPrintAnalysis_formatsOutput(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	a := &ai.MealAnalysis{
		Title:       "Grilled Salmon",
		Description: "A serving of salmon fillet.",
		Items:       []string{"salmon", "lemon"},
		Kcal:        350,
		ProteinG:    40,
		CarbsG:      2,
		FatG:        18,
		Confidence:  "high",
	}
	require.NoError(t, printAnalysis(cmd, a))

	out := buf.String()
	for _, want := range []string{"Grilled Salmon", "350", "40", "salmon", "high"} {
		assert.Contains(t, out, want)
	}
}

func TestPrintAnalysis_emptyFields(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	a := &ai.MealAnalysis{Title: "X", Kcal: 100}
	require.NoError(t, printAnalysis(cmd, a))
	assert.Contains(t, buf.String(), "X")
}

const analyzeMealJSON = `{"title":"pasta","description":"pasta bolognese","items":["pasta","beef"],"kcal":600,"protein_g":30,"carbs_g":70,"fat_g":20,"confidence":"high"}`

func TestAnalyzeAndPrint_success(t *testing.T) {
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: analyzeMealJSON},
		{Done: true, Final: analyzeMealJSON},
	}}
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := analyzeAndPrint(context.Background(), cmd, client, "", "a plate of pasta", []string{}, nil)
	require.NoError(t, err)
	out := buf.String()
	assert.Contains(t, out, "pasta")
	assert.Contains(t, out, "600")
}

func TestAnalyzeAndPrint_noInput(t *testing.T) {
	client := &mockStreamer{}
	cmd := &cobra.Command{}
	cmd.Flags().StringP("message", "m", "", "message")
	err := analyzeAndPrint(context.Background(), cmd, client, "", "", nil, nil)
	require.Error(t, err)
}

func TestAnalyzeAndPrint_withURLAndPath(t *testing.T) {
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: analyzeMealJSON},
		{Done: true, Final: analyzeMealJSON},
	}}
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	args := []string{"https://example.com/food.jpg", "/local/path.jpg"}
	err := analyzeAndPrint(context.Background(), cmd, client, "", "lunch", args, nil)
	require.NoError(t, err)
}

func TestAnalyzeAndPrint_analyzeError(t *testing.T) {
	client := &mockStreamer{err: errors.New("ai down")}
	cmd := &cobra.Command{}
	err := analyzeAndPrint(context.Background(), cmd, client, "", "pasta", []string{}, nil)
	require.Error(t, err)
}

func TestRunAnalyze_missingKey(t *testing.T) {
	// runAnalyze with missing API key → openAIClient fails.
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("message", "m", "", "message")
	cmd.Flags().BoolP("log", "l", false, "log")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runAnalyze(cmd, []string{"meal.jpg"})
	require.Error(t, err)
}

func TestRunAnalyze_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("message", "m", "", "message")
	cmd.Flags().BoolP("log", "l", false, "log")

	err := runAnalyze(cmd, []string{"meal.jpg"})
	require.Error(t, err)
}

func TestRunAnalyze_noInput(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("message", "m", "", "message")
	cmd.Flags().BoolP("log", "l", false, "log")

	err := runAnalyze(cmd, nil) // no args, no -m flag
	require.Error(t, err)
}

func TestRunAnalyze_logFlagNoInput(t *testing.T) {
	// --log=true with a valid store but no media/text: the store opens (store = s
	// is executed) and then analyzeAndPrint fails with "nothing to analyze".
	// This covers the defer s.Close() + store = s lines inside the if-log branch.
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("message", "m", "", "message")
	cmd.Flags().BoolP("log", "l", false, "log")
	_ = cmd.Flags().Set("log", "true")

	err := runAnalyze(cmd, nil) // no args, no -m — fails at analyzeAndPrint
	require.Error(t, err)
}

func TestAnalyzeAndPrint_withStore_savesResult(t *testing.T) {
	rec := &saveMealRecorder{}
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: analyzeMealJSON},
		{Done: true, Final: analyzeMealJSON},
	}}
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := analyzeAndPrint(context.Background(), cmd, client, "", "pasta", []string{}, rec)
	require.NoError(t, err)
	require.Len(t, rec.saved, 1)
	assert.Equal(t, "pasta", rec.saved[0].Title)
}

func TestRunAnalyze_logFlag_storeError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based store error not applicable on Windows")
	}
	if os.Getuid() == 0 {
		t.Skip("running as root — chmod cannot restrict access")
	}

	// Create a read-only directory so SQLite cannot create the DB file.
	dir := t.TempDir()
	require.NoError(t, os.Chmod(dir, 0o444))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("BITE_DB", filepath.Join(dir, "bite.db"))
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().StringP("message", "m", "", "message")
	cmd.Flags().BoolP("log", "l", false, "log")
	_ = cmd.Flags().Set("log", "true")

	err := runAnalyze(cmd, []string{"meal.jpg"})
	require.Error(t, err)
}

func TestAnalyzeAndPrint_withStore_saveError_continuesPrint(t *testing.T) {
	// When SaveMeal fails, the error is printed to stderr but analysis is still
	// printed to stdout.
	rec := &saveMealRecorder{err: errors.New("disk full")}
	client := &mockStreamer{events: []ai.StreamEvent{
		{Delta: analyzeMealJSON},
		{Done: true, Final: analyzeMealJSON},
	}}
	cmd := &cobra.Command{}
	var outBuf, errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	err := analyzeAndPrint(context.Background(), cmd, client, "", "pasta", []string{}, rec)
	require.NoError(t, err)
	assert.Contains(t, outBuf.String(), "pasta")
	assert.Contains(t, errBuf.String(), "disk full")
}
