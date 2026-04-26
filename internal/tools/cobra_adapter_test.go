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

func TestCountingWriter_tracksBytes(t *testing.T) {
	var buf bytes.Buffer
	cw := &countingWriter{w: &buf}
	n, err := cw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, 5, cw.n)
	assert.Equal(t, "hello", buf.String())

	_, _ = cw.Write([]byte(" world"))
	assert.Equal(t, 11, cw.n)
}

func TestRegisterCobra_depsProviderError(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("p"))
		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, func(_ context.Context) (Deps, func(), error) {
			return Deps{}, nil, assert.AnError
		})
		_, err := runCmd(t, root, "p")
		require.Error(t, err)
	})
}

func TestRegisterCobra_cleanupRuns(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("p"))
		var called bool
		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, func(_ context.Context) (Deps, func(), error) {
			return Deps{}, func() { called = true }, nil
		})
		_, err := runCmd(t, root, "p")
		require.NoError(t, err)
		assert.True(t, called, "cleanup should run after Run completes")
	})
}

func TestParseString_allTypes(t *testing.T) {
	v, err := parseString(ParamString, "hi")
	require.NoError(t, err)
	assert.Equal(t, "hi", v)

	v, err = parseString(ParamInt, "42")
	require.NoError(t, err)
	assert.Equal(t, int64(42), v)

	v, err = parseString(ParamFloat, "3.14")
	require.NoError(t, err)
	assert.Equal(t, 3.14, v)

	v, err = parseString(ParamBool, "true")
	require.NoError(t, err)
	assert.Equal(t, true, v)

	v, err = parseString(ParamStringList, "x")
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, v)
}

func TestParseString_invalidValues(t *testing.T) {
	_, err := parseString(ParamInt, "abc")
	assert.Error(t, err)
	_, err = parseString(ParamFloat, "abc")
	assert.Error(t, err)
	_, err = parseString(ParamBool, "yesyes")
	assert.Error(t, err)
}

func TestRegisterCobra_bindsAllFlagTypes(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name: "all_types", Summary: "s", Description: "d",
			Params: []Param{
				{Name: "s", Type: ParamString, Default: "default-s"},
				{Name: "i", Type: ParamInt, Default: int64(7)},
				{Name: "f", Type: ParamFloat, Default: 1.5},
				{Name: "b", Type: ParamBool, Default: true},
				{Name: "xs", Type: ParamStringList},
			},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})

		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))

		_, err := runCmd(t, root, "all_types",
			"--s=hello", "--i=99", "--f=2.5", "--b=false",
			"--xs=a", "--xs=b")
		require.NoError(t, err)
		assert.Equal(t, "hello", seen.String("s"))
		assert.Equal(t, int64(99), seen.Int("i"))
		assert.Equal(t, 2.5, seen.Float("f"))
		assert.False(t, seen.Bool("b"))
		assert.Equal(t, []string{"a", "b"}, seen.StringList("xs"))
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
