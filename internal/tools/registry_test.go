package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noopTool returns a minimal valid Tool for invariant tests.
func noopTool(name string) Tool {
	return Tool{
		Name:        name,
		Summary:     "summary",
		Description: "description",
		Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
			return Result{Text: "ok"}, nil
		},
	}
}

// ─── live-registry invariants ────────────────────────────────────────────────
//
// These iterate the live registry — adding a tool extends coverage automatically.
// No hardcoded list of tool names: that ban is the whole point of this package.

func TestRegistry_invariants(t *testing.T) {
	for _, tool := range All() {
		require.NoError(t, tool.validate(), "tool %q failed invariants", tool.Name)
	}
}

func TestRegistry_namesUnique(t *testing.T) {
	seen := map[string]struct{}{}
	for _, tool := range All() {
		_, dup := seen[tool.Name]
		assert.False(t, dup, "tool name %q registered twice", tool.Name)
		seen[tool.Name] = struct{}{}
	}
}

// ─── isolated tests against a swappable registry ─────────────────────────────

func withCleanRegistry(t *testing.T, fn func()) {
	t.Helper()
	regMu.Lock()
	saved := reg
	reg = map[string]Tool{}
	regMu.Unlock()
	t.Cleanup(func() {
		regMu.Lock()
		reg = saved
		regMu.Unlock()
	})
	fn()
}

func TestRegister_and_All(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("b_tool"))
		Register(noopTool("a_tool"))
		got := All()
		require.Len(t, got, 2)
		assert.Equal(t, "a_tool", got[0].Name, "All() should sort by name")
		assert.Equal(t, "b_tool", got[1].Name)
	})
}

func TestRegister_panicsOnDuplicate(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("dup"))
		assert.Panics(t, func() { Register(noopTool("dup")) })
	})
}

func TestRegister_panicsOnInvalid(t *testing.T) {
	withCleanRegistry(t, func() {
		assert.Panics(t, func() { Register(Tool{Name: "x", Summary: "s", Run: nil, Description: "d"}) })
		assert.Panics(t, func() { Register(Tool{Name: "BAD", Summary: "s", Description: "d", Run: noopTool("x").Run}) })
		assert.Panics(t, func() { Register(Tool{Name: "no_summary", Description: "d", Run: noopTool("x").Run}) })
	})
}

func TestMustGet_panicsWhenMissing(t *testing.T) {
	withCleanRegistry(t, func() {
		assert.Panics(t, func() { MustGet("nope") })
	})
}

func TestGet_returnsRegistered(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("hi"))
		got, ok := Get("hi")
		require.True(t, ok)
		assert.Equal(t, "hi", got.Name)
	})
}

// ─── validate edge cases ─────────────────────────────────────────────────────

func TestValidate_requiredPositionalAfterOptional_fails(t *testing.T) {
	tt := noopTool("bad_order")
	tt.Params = []Param{
		{Name: "opt", Type: ParamString, Positional: true, Required: false},
		{Name: "req", Type: ParamString, Positional: true, Required: true},
	}
	assert.Error(t, tt.validate())
}

func TestValidate_duplicateParam_fails(t *testing.T) {
	tt := noopTool("dup_param")
	tt.Params = []Param{
		{Name: "x", Type: ParamString},
		{Name: "x", Type: ParamInt},
	}
	assert.Error(t, tt.validate())
}

func TestValidate_paramNameNotSnakeCase_fails(t *testing.T) {
	tt := noopTool("bad_param_name")
	tt.Params = []Param{{Name: "Foo", Type: ParamString}}
	assert.Error(t, tt.validate())
}

func TestValidate_acceptsCleanTool(t *testing.T) {
	tt := noopTool("good")
	tt.Params = []Param{
		{Name: "req_pos", Type: ParamString, Positional: true, Required: true},
		{Name: "opt_pos", Type: ParamString, Positional: true},
		{Name: "opt_flag", Type: ParamFloat},
	}
	assert.NoError(t, tt.validate())
}

func TestArgs_accessors(t *testing.T) {
	a := NewArgs(map[string]any{
		"s":  "hi",
		"i":  float64(42), // JSON numbers arrive as float64
		"f":  3.14,
		"b":  true,
		"sl": []any{"a", "b"},
	})
	assert.True(t, a.Has("s"))
	assert.False(t, a.Has("missing"))
	assert.Equal(t, "hi", a.String("s"))
	assert.Equal(t, int64(42), a.Int("i"))
	assert.Equal(t, 3.14, a.Float("f"))
	assert.True(t, a.Bool("b"))
	assert.Equal(t, []string{"a", "b"}, a.StringList("sl"))
}

func TestArgs_StringList_acceptsNativeSlice(t *testing.T) {
	// AI/JSON inputs arrive as []any, but cobra's StringArray flags hand us
	// a real []string. Both must round-trip identically.
	a := NewArgs(map[string]any{"xs": []string{"a", "b"}})
	assert.Equal(t, []string{"a", "b"}, a.StringList("xs"))
}

func TestArgs_StringList_filtersWrongTypes(t *testing.T) {
	// Stray non-string entries (e.g. JSON number in a list-of-strings) get
	// dropped rather than panic the tool.
	a := NewArgs(map[string]any{"xs": []any{"a", 42, "b"}})
	assert.Equal(t, []string{"a", "b"}, a.StringList("xs"))
}

func TestArgs_FloatAcceptsAllNumericTypes(t *testing.T) {
	a := NewArgs(map[string]any{
		"f64": float64(1.5),
		"f32": float32(2.5),
		"i":   3,
		"i64": int64(4),
	})
	assert.Equal(t, 1.5, a.Float("f64"))
	assert.Equal(t, 2.5, a.Float("f32"))
	assert.Equal(t, 3.0, a.Float("i"))
	assert.Equal(t, 4.0, a.Float("i64"))
}

func TestArgs_IntAcceptsAllNumericTypes(t *testing.T) {
	a := NewArgs(map[string]any{
		"i":   7,
		"i64": int64(8),
		"f32": float32(9.7),
		"f64": 10.4,
	})
	assert.Equal(t, int64(7), a.Int("i"))
	assert.Equal(t, int64(8), a.Int("i64"))
	assert.Equal(t, int64(9), a.Int("f32"))
	assert.Equal(t, int64(10), a.Int("f64"))
}

func TestArgs_typeMismatchReturnsZero(t *testing.T) {
	a := NewArgs(map[string]any{
		"s": 42,         // not a string
		"i": "no",       // not a number
		"f": "no",       // not a number
		"b": "no",       // not a bool
		"x": struct{}{}, // not a list
	})
	assert.Equal(t, "", a.String("s"))
	assert.Equal(t, int64(0), a.Int("i"))
	assert.Equal(t, 0.0, a.Float("f"))
	assert.False(t, a.Bool("b"))
	assert.Nil(t, a.StringList("x"))
}

func TestArgs_zeroOnMissing(t *testing.T) {
	a := NewArgs(nil)
	assert.Equal(t, "", a.String("x"))
	assert.Equal(t, int64(0), a.Int("x"))
	assert.Equal(t, 0.0, a.Float("x"))
	assert.False(t, a.Bool("x"))
	assert.Nil(t, a.StringList("x"))
}

func TestDeps_NowOrDefault(t *testing.T) {
	// Production path: Deps.Now is nil, NowOrDefault returns time.Now().
	got := (Deps{}).NowOrDefault()
	assert.WithinDuration(t, time.Now(), got, time.Second)

	// Test path: Deps.Now returns a fixed value.
	fixed := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	d := Deps{Now: func() time.Time { return fixed }}
	assert.Equal(t, fixed, d.NowOrDefault())
}

func TestDeps_LocOrDefault(t *testing.T) {
	assert.Equal(t, time.Local, (Deps{}).LocOrDefault())
	d := Deps{Loc: time.UTC}
	assert.Equal(t, time.UTC, d.LocOrDefault())
}

func TestFlagName_kebabsUnderscores(t *testing.T) {
	assert.Equal(t, "kcal", flagName(Param{Name: "kcal"}))
	assert.Equal(t, "protein-g", flagName(Param{Name: "protein_g"}))
	assert.Equal(t, "log-meal-from-media", flagName(Param{Name: "log_meal_from_media"}))
}

// isSnakeCase exhaustively — these guards stop garbage names from leaking
// into AI tool specs and CLI subcommands, so the rules need explicit tests.
func TestIsSnakeCase(t *testing.T) {
	cases := map[string]bool{
		"":                    false, // empty
		"foo":                 true,
		"foo_bar":             true,
		"foo_bar_baz":         true,
		"foo123":              true,
		"foo_bar_42":          true,
		"_foo":                false, // leading underscore
		"foo_":                false, // trailing underscore
		"123foo":              false, // leading digit
		"Foo":                 false, // uppercase
		"FOO":                 false,
		"foo-bar":             false, // hyphen
		"foo bar":             false, // space
		"foo.bar":             false, // dot
		"foo$":                false, // special char
		"meals_today":         true,
		"log_meal":            true,
		"log_meal_from_media": true,
	}
	for input, want := range cases {
		assert.Equal(t, want, isSnakeCase(input), "isSnakeCase(%q)", input)
	}
}

func TestValidate_emptyName_fails(t *testing.T) {
	tt := noopTool("ok")
	tt.Name = ""
	assert.Error(t, tt.validate())
}

func TestValidate_nilRun_fails(t *testing.T) {
	tt := noopTool("ok")
	tt.Run = nil
	assert.Error(t, tt.validate())
}

func TestValidate_emptySummary_fails(t *testing.T) {
	tt := noopTool("ok")
	tt.Summary = ""
	assert.Error(t, tt.validate())
}

func TestValidate_emptyDescription_fails(t *testing.T) {
	tt := noopTool("ok")
	tt.Description = ""
	assert.Error(t, tt.validate())
}

func TestValidate_emptyParamName_fails(t *testing.T) {
	tt := noopTool("ok")
	tt.Params = []Param{{Name: "", Type: ParamString}}
	assert.Error(t, tt.validate())
}

func TestValidate_reservedName_fails(t *testing.T) {
	// "help" is owned by the slash dispatcher — registering a tool with
	// that name would silently shadow it from the user.
	assert.Error(t, noopTool("help").validate())
}

func TestValidate_defaultTypeMismatch_fails(t *testing.T) {
	tt := noopTool("bad_default")
	tt.Params = []Param{{Name: "n", Type: ParamInt, Default: "not-an-int"}}
	assert.Error(t, tt.validate())
}

func TestTool_Long_prefersDescribeDynamic(t *testing.T) {
	tt := noopTool("d")
	tt.Description = "static"
	tt.DescribeDynamic = func() string { return "dynamic" }
	assert.Equal(t, "dynamic", tt.Long())
}

func TestTool_Long_fallsBackToDescription(t *testing.T) {
	tt := noopTool("d")
	tt.Description = "static"
	// DescribeDynamic returns empty → fall back to Description rather than
	// emitting a blank help text.
	tt.DescribeDynamic = func() string { return "" }
	assert.Equal(t, "static", tt.Long())
}

func TestTool_Long_noDynamicReturnsDescription(t *testing.T) {
	tt := noopTool("d")
	tt.Description = "static"
	assert.Equal(t, "static", tt.Long())
}

func TestValidate_requiredWithDefault_fails(t *testing.T) {
	// Required + Default is contradictory — the default is unreachable.
	tt := noopTool("contradict")
	tt.Params = []Param{{Name: "x", Type: ParamString, Required: true, Default: "fallback"}}
	assert.Error(t, tt.validate())
}

func TestValidate_defaultTypeMatch_passes(t *testing.T) {
	tt := noopTool("good_default")
	tt.Params = []Param{
		{Name: "s", Type: ParamString, Default: "hi"},
		{Name: "i", Type: ParamInt, Default: int64(7)},
		{Name: "f", Type: ParamFloat, Default: 3.14},
		{Name: "b", Type: ParamBool, Default: true},
		{Name: "xs", Type: ParamStringList, Default: []string{"a"}},
	}
	assert.NoError(t, tt.validate())
}
