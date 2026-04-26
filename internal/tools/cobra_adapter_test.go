package tools

import (
	"bytes"
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runCmd executes a cobra root with the given args and returns combined output.
func runCmd(t *testing.T, root *cobra.Command, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	root.SetContext(context.Background())
	err := root.Execute()
	return buf.String(), err
}

func TestRegisterCobra_addsCommandPerTool(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("alpha"))
		Register(noopTool("beta"))

		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))

		var found []string
		for _, c := range root.Commands() {
			found = append(found, c.Use)
		}
		assert.Contains(t, found, "alpha")
		assert.Contains(t, found, "beta")
	})
}

func TestRegisterCobra_helpRendersForEveryTool(t *testing.T) {
	for _, tool := range All() {
		t.Run(tool.Name, func(t *testing.T) {
			root := &cobra.Command{Use: "test"}
			RegisterCobra(root, StaticDeps(Deps{}))
			out, err := runCmd(t, root, tool.Name, "--help")
			require.NoError(t, err)
			// First line of Description shows in --help.
			firstLine := tool.Description
			if idx := indexOfNewline(firstLine); idx > 0 {
				firstLine = firstLine[:idx]
			}
			assert.Contains(t, out, firstLine)
		})
	}
}

func indexOfNewline(s string) int {
	for i, r := range s {
		if r == '\n' {
			return i
		}
	}
	return -1
}

func TestRegisterCobra_positionalParam(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name:        "echo_pos",
			Summary:     "s",
			Description: "d",
			Params: []Param{
				{Name: "id", Type: ParamInt, Required: true, Positional: true},
			},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})

		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))

		out, err := runCmd(t, root, "echo_pos", "42")
		require.NoError(t, err)
		assert.Contains(t, out, "ok")
		assert.Equal(t, int64(42), seen.Int("id"))
	})
}

func TestRegisterCobra_flagParam(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name:        "set_kcal",
			Summary:     "s",
			Description: "d",
			Params: []Param{
				{Name: "kcal", Type: ParamFloat},
				{Name: "protein_g", Type: ParamFloat},
			},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})

		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))

		_, err := runCmd(t, root, "set_kcal", "--kcal=2000", "--protein-g=150")
		require.NoError(t, err)
		assert.True(t, seen.Has("kcal"))
		assert.Equal(t, 2000.0, seen.Float("kcal"))
		assert.True(t, seen.Has("protein_g"))
		assert.Equal(t, 150.0, seen.Float("protein_g"))
	})
}

func TestRegisterCobra_unchangedFlagAbsent(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name:        "maybe",
			Summary:     "s",
			Description: "d",
			Params:      []Param{{Name: "kcal", Type: ParamFloat}},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})

		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))

		_, err := runCmd(t, root, "maybe")
		require.NoError(t, err)
		assert.False(t, seen.Has("kcal"), "unset flag should not appear in args")
	})
}

func TestRegisterCobra_invalidPositional_errors(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name:        "need_int",
			Summary:     "s",
			Description: "d",
			Params: []Param{
				{Name: "n", Type: ParamInt, Required: true, Positional: true},
			},
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
				return Result{Text: "ok"}, nil
			},
		})

		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))

		_, err := runCmd(t, root, "need_int", "abc")
		require.Error(t, err)
	})
}

func TestRenderForCobra_textAndTable(t *testing.T) {
	var buf bytes.Buffer
	renderForCobra(&buf, Result{
		Text: "hello",
		Table: &Table{
			Headers: []string{"A", "B"},
			Rows:    [][]string{{"1", "2"}},
			Footer:  []string{"sum", "1"},
		},
	})
	out := buf.String()
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "A")
	assert.Contains(t, out, "B")
	assert.Contains(t, out, "1")
	assert.Contains(t, out, "sum")
}
