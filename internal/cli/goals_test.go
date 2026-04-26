package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fp returns a *float64 pointer (helper for test readability).
func fp(v float64) *float64 { return &v }

// goalCmd builds a bare command with all goals flags registered.
func goalCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Float64P("kcal", "k", -1, "")
	cmd.Flags().Float64P("protein", "p", -1, "")
	cmd.Flags().Float64P("carbs", "c", -1, "")
	cmd.Flags().Float64P("fat", "f", -1, "")
	return cmd
}

// ─── applyFlag tests ──────────────────────────────────────────────────────────

func TestApplyFlag_unchanged(t *testing.T) {
	cmd := goalCmd()
	v := fp(2000)
	applyFlag(cmd, "kcal", &v)
	require.NotNil(t, v)
	assert.Equal(t, 2000.0, *v)
}

func TestApplyFlag_set(t *testing.T) {
	cmd := goalCmd()
	_ = cmd.Flags().Set("kcal", "1800")
	var v *float64
	applyFlag(cmd, "kcal", &v)
	require.NotNil(t, v)
	assert.Equal(t, 1800.0, *v)
}

func TestApplyFlag_clear(t *testing.T) {
	cmd := goalCmd()
	_ = cmd.Flags().Set("kcal", "0")
	v := fp(2000)
	applyFlag(cmd, "kcal", &v)
	assert.Nil(t, v, "expected nil after clear")
}

// ─── printGoalField tests ─────────────────────────────────────────────────────

func TestPrintGoalField_nil(t *testing.T) {
	var buf bytes.Buffer
	printGoalField(&buf, "  kcal    ", nil)
	assert.Contains(t, buf.String(), "—")
}

func TestPrintGoalField_set(t *testing.T) {
	var buf bytes.Buffer
	printGoalField(&buf, "  kcal    ", fp(2000))
	assert.Contains(t, buf.String(), "2000")
}

// ─── handleGoals / showGoals tests ───────────────────────────────────────────

func TestHandleGoals_noFlags_showsDefaults(t *testing.T) {
	store := newMockStore()
	cmd := goalCmd()
	var buf bytes.Buffer

	require.NoError(t, handleGoals(context.Background(), &buf, cmd, store))
	out := buf.String()
	assert.Contains(t, out, "Daily targets:")
	assert.Contains(t, out, "—")
}

func TestHandleGoals_setKcal(t *testing.T) {
	store := newMockStore()
	cmd := goalCmd()
	_ = cmd.Flags().Set("kcal", "2000")
	var buf bytes.Buffer

	require.NoError(t, handleGoals(context.Background(), &buf, cmd, store))
	assert.Contains(t, buf.String(), "2000")
	require.NotNil(t, store.goals.Kcal)
	assert.Equal(t, 2000.0, *store.goals.Kcal)
}

func TestHandleGoals_clearKcal(t *testing.T) {
	store := newMockStore()
	store.goals.Kcal = fp(1800)
	cmd := goalCmd()
	_ = cmd.Flags().Set("kcal", "0")

	require.NoError(t, handleGoals(context.Background(), &bytes.Buffer{}, cmd, store))
	assert.Nil(t, store.goals.Kcal, "expected Kcal nil after clear")
}

func TestHandleGoals_allFlags(t *testing.T) {
	store := newMockStore()
	cmd := goalCmd()
	for name, val := range map[string]string{
		"kcal": "2200", "protein": "160", "carbs": "220", "fat": "70",
	} {
		_ = cmd.Flags().Set(name, val)
	}

	require.NoError(t, handleGoals(context.Background(), &bytes.Buffer{}, cmd, store))
	require.NotNil(t, store.goals.ProteinG)
	assert.Equal(t, 160.0, *store.goals.ProteinG)
	require.NotNil(t, store.goals.CarbsG)
	assert.Equal(t, 220.0, *store.goals.CarbsG)
	require.NotNil(t, store.goals.FatG)
	assert.Equal(t, 70.0, *store.goals.FatG)
}

func TestHandleGoals_getGoalsError_noFlags(t *testing.T) {
	// No flags → goes through showGoals → GetGoals fails.
	store := newMockStore()
	store.goalsErr = errors.New("db offline")
	cmd := goalCmd()
	err := handleGoals(context.Background(), &bytes.Buffer{}, cmd, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db offline")
}

func TestHandleGoals_getGoalsError_withFlag(t *testing.T) {
	// Flag set → GetGoals called to load current values before merge; that fails.
	store := newMockStore()
	store.goalsErr = errors.New("db offline")
	cmd := goalCmd()
	_ = cmd.Flags().Set("kcal", "2000")
	err := handleGoals(context.Background(), &bytes.Buffer{}, cmd, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db offline")
}

func TestHandleGoals_setGoalsError(t *testing.T) {
	store := newMockStore()
	store.setGoalsErr = errors.New("write failed")
	cmd := goalCmd()
	_ = cmd.Flags().Set("kcal", "2000")
	err := handleGoals(context.Background(), &bytes.Buffer{}, cmd, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

func TestShowGoals_storeError(t *testing.T) {
	store := newMockStore()
	store.goalsErr = errors.New("db offline")
	err := showGoals(context.Background(), &bytes.Buffer{}, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db offline")
}

func TestRunGoals_integration(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Float64P("kcal", "k", -1, "")
	cmd.Flags().Float64P("protein", "p", -1, "")
	cmd.Flags().Float64P("carbs", "c", -1, "")
	cmd.Flags().Float64P("fat", "f", -1, "")
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runGoals(cmd, nil))
	assert.Contains(t, buf.String(), "Daily targets:")
}

func TestRunGoals_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.Flags().Float64P("kcal", "k", -1, "")
	cmd.Flags().Float64P("protein", "p", -1, "")
	cmd.Flags().Float64P("carbs", "c", -1, "")
	cmd.Flags().Float64P("fat", "f", -1, "")

	err := runGoals(cmd, nil)
	require.Error(t, err)
}
