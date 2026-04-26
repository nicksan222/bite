package tools

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecks_returnsRegistered(t *testing.T) {
	got := Checks()
	assert.NotEmpty(t, got, "expected built-in checks to be registered")

	// Hard severity must come first.
	for i := 1; i < len(got); i++ {
		assert.LessOrEqual(t, int(got[i-1].Severity), int(got[i].Severity),
			"checks must be sorted with Hard before Soft")
	}
}

func TestRegisterCheck_panicsOnInvalid(t *testing.T) {
	assert.Panics(t, func() { RegisterCheck(Check{Name: ""}) })
	assert.Panics(t, func() { RegisterCheck(Check{Name: "x", Run: nil}) })
}

// snapshotCheckRegistry preserves the live check registry around a test
// that mutates it (e.g. via RegisterCheck). Restores both the slice and the
// dedup set on cleanup so the rest of the package's tests see the original
// shape.
func snapshotCheckRegistry(t *testing.T) {
	t.Helper()
	checkMu.Lock()
	saved := append([]Check(nil), checks...)
	savedSet := maps.Clone(checked)
	checkMu.Unlock()
	t.Cleanup(func() {
		checkMu.Lock()
		checks = saved
		checked = savedSet
		checkMu.Unlock()
	})
}

func TestRegisterCheck_panicsOnDuplicate(t *testing.T) {
	snapshotCheckRegistry(t)
	assert.Panics(t, func() {
		RegisterCheck(Check{Name: "config: load", Run: func(_ context.Context) (string, error) { return "", nil }})
	})
}

func TestDoctorDescription_listsEveryCheck(t *testing.T) {
	desc := doctorDescription()
	for _, c := range Checks() {
		assert.Contains(t, desc, c.Name, "description should mention %q", c.Name)
	}
}

func TestDoctorDescription_marksGated(t *testing.T) {
	desc := doctorDescription()
	for _, c := range Checks() {
		if c.Gate != "" {
			assert.Contains(t, desc, "--"+c.Gate, "gated check %q should mention --%s", c.Name, c.Gate)
		}
	}
}

func TestDoctor_runsAllUnskippedChecks(t *testing.T) {
	res, err := MustGet("doctor").Run(context.Background(), Deps{}, NewArgs(map[string]any{
		"ping": false,
	}))
	// We don't care about pass/fail in this test; we care that every non-gated
	// check ran. Each check writes its name to the result text.
	_ = err
	for _, c := range Checks() {
		if c.Gate != "" {
			assert.NotContains(t, res.Text, "  ✓ "+c.Name, "gated check %q should NOT run", c.Name)
			continue
		}
		assert.Contains(t, res.Text, c.Name, "check %q should appear in output", c.Name)
	}
}

func TestDoctor_allHardChecksPassWithStubEnv(t *testing.T) {
	// Force every non-ping hard check down its success path: valid config,
	// valid SQLite path, and an API key are enough.
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("OPENAI_API_KEY", "sk-openai-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	res, err := MustGet("doctor").Run(context.Background(), Deps{}, NewArgs(map[string]any{
		"ping": false,
	}))
	require.NoError(t, err, "doctor output:\n%s", res.Text)
	assert.Contains(t, res.Text, "All required checks passed")
	assert.Contains(t, res.Text, "audio transcription available")
}

func TestDoctor_pingGateRunsCheck(t *testing.T) {
	// Find the ping-gated check.
	var gated []string
	for _, c := range Checks() {
		if c.Gate == "ping" {
			gated = append(gated, c.Name)
		}
	}
	require.NotEmpty(t, gated, "expected at least one ping-gated check")

	res, _ := MustGet("doctor").Run(context.Background(), Deps{}, NewArgs(map[string]any{
		"ping": true,
	}))
	for _, name := range gated {
		assert.Contains(t, res.Text, name, "ping=true should run gated check %q", name)
	}
}

func TestRunCheck_recordsFailure(t *testing.T) {
	var sb strings.Builder
	ok := runCheck(context.Background(), &sb, Check{
		Name: "fails",
		Run:  func(_ context.Context) (string, error) { return "", errors.New("boom") },
	}, "✗")
	assert.False(t, ok)
	assert.Contains(t, sb.String(), "fails")
	assert.Contains(t, sb.String(), "boom")
}

func TestSeverity_String(t *testing.T) {
	assert.Equal(t, "hard", SeverityHard.String())
	assert.Equal(t, "soft", SeveritySoft.String())
	assert.Equal(t, "unknown", Severity(99).String())
}

func TestDescribeCheck_fallsBackToName(t *testing.T) {
	var b strings.Builder
	describeCheck(&b, Check{Name: "tagged", Gate: "ping"})
	out := b.String()
	assert.Contains(t, out, "tagged")
	assert.Contains(t, out, "(only with --ping)")
}

func TestRunCheck_recordsSuccess(t *testing.T) {
	var sb strings.Builder
	ok := runCheck(context.Background(), &sb, Check{
		Name: "passes",
		Run:  func(_ context.Context) (string, error) { return "all good", nil },
	}, "✗")
	assert.True(t, ok)
	assert.Contains(t, sb.String(), "passes")
	assert.Contains(t, sb.String(), "all good")
}

func TestChecks_orderStable(t *testing.T) {
	a := Checks()
	b := Checks()
	require.Equal(t, len(a), len(b))
	namesA := make([]string, len(a))
	namesB := make([]string, len(b))
	for i := range a {
		namesA[i] = a[i].Name
		namesB[i] = b[i].Name
	}
	assert.Equal(t, namesA, namesB, "Checks() must return the same order on repeated calls")
	for i := 1; i < len(a); i++ {
		assert.LessOrEqual(t, int(a[i-1].Severity), int(a[i].Severity),
			"Checks must be sorted by severity (hard before soft)")
	}
}
