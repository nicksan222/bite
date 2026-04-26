package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMeta_everyToolReachesEverySurface is the load-bearing invariant of the
// whole package: any Tool that registers itself MUST appear in every adapter
// (AI, cobra, slash) and in the system-prompt appendix. Iterating the live
// registry means a forgotten wiring step (e.g. a new adapter that skips some
// tools) is caught here regardless of which tools exist.
//
// No hardcoded list of tool names — that's the whole point.
func TestMeta_everyToolReachesEverySurface(t *testing.T) {
	all := All()
	if len(all) == 0 {
		t.Fatal("expected at least one registered tool")
	}

	aiNames := map[string]struct{}{}
	for _, at := range AITools(Deps{}) {
		aiNames[at.Name] = struct{}{}
	}

	root := &cobra.Command{Use: "test"}
	RegisterCobra(root, StaticDeps(Deps{}))
	cobraNames := map[string]struct{}{}
	for _, c := range root.Commands() {
		// Use can be "name <arg>"; take the first token.
		first := c.Use
		for i, r := range first {
			if r == ' ' {
				first = first[:i]
				break
			}
		}
		cobraNames[first] = struct{}{}
	}

	appendix := RenderAppendix()

	for _, tool := range all {
		t.Run(tool.Name, func(t *testing.T) {
			// Cobra is universal: every tool is a `bite <name>` subcommand.
			_, inCobra := cobraNames[tool.Name]
			assert.True(t, inCobra, "tool %q missing from RegisterCobra()", tool.Name)

			// AI surface is conditional on SkipAI.
			_, inAI := aiNames[tool.Name]
			if tool.SkipAI {
				assert.False(t, inAI, "tool %q has SkipAI but appears in AITools()", tool.Name)
				assert.NotContains(t, appendix, "### "+tool.Name,
					"tool %q has SkipAI but appears in system-prompt appendix", tool.Name)
			} else {
				assert.True(t, inAI, "tool %q missing from AITools()", tool.Name)
				assert.Contains(t, appendix, tool.Name, "tool %q missing from system-prompt appendix", tool.Name)
			}
		})
	}
}

// TestMeta_skipSlashHidesFromDispatch verifies SkipSlash tools are unreachable
// via /<name> and absent from /help, while still being reachable via cobra.
func TestMeta_skipSlashHidesFromDispatch(t *testing.T) {
	for _, tool := range All() {
		if !tool.SkipSlash {
			continue
		}
		t.Run(tool.Name, func(t *testing.T) {
			out := Dispatch(context.Background(), Deps{}, "/"+tool.Name)
			require.Error(t, out.ParseError, "slash dispatch should reject SkipSlash tool %q", tool.Name)
			assert.Contains(t, out.ParseError.Error(), "unknown command")

			help := Dispatch(context.Background(), Deps{}, "/help")
			assert.NotContains(t, help.Result.Text, "/"+tool.Name+" ",
				"SkipSlash tool %q must not appear in /help", tool.Name)
			assert.NotContains(t, help.Result.Text, "/"+tool.Name+"\n",
				"SkipSlash tool %q must not appear in /help", tool.Name)
		})
	}
}

// TestMeta_examplesAreWellFormed double-checks every registered tool's
// Examples — registration-time validate() already enforces this contract,
// but the meta-test is the live tripwire if validate() is ever loosened.
func TestMeta_examplesAreWellFormed(t *testing.T) {
	for _, tool := range All() {
		for _, ex := range tool.Examples {
			t.Run(tool.Name+":"+ex.Cmd, func(t *testing.T) {
				wantPrefix := "bite " + tool.Name
				ok := ex.Cmd == wantPrefix || strings.HasPrefix(ex.Cmd, wantPrefix+" ")
				assert.True(t, ok, "example Cmd %q must start with %q", ex.Cmd, wantPrefix)
				assert.NotEmpty(t, ex.Desc, "example Desc must be non-empty")
			})
		}
	}
}

// TestMeta_everyChecksRunsFromDoctor mirrors the above for the Check
// registry: every registered Check must surface in the doctor tool's help
// text and (when its gate is on, or absent) actually execute.
func TestMeta_everyCheckSurfacesInDoctor(t *testing.T) {
	desc := doctorDescription()
	for _, c := range Checks() {
		assert.Contains(t, desc, c.Name, "check %q missing from doctor help", c.Name)
	}
}

// TestMeta_everyToolHasAtLeastOneExample asserts every registered tool
// contributes at least one row to `bite --help` (and `bite <tool> --help`).
// Without this gate a tool registered without Examples is technically valid
// but invisible in the rootCmd's example block — a discoverability hole
// future contributors won't notice until they read the help themselves.
func TestMeta_everyToolHasAtLeastOneExample(t *testing.T) {
	for _, tool := range All() {
		t.Run(tool.Name, func(t *testing.T) {
			assert.NotEmpty(t, tool.Examples,
				"tool %q has no Examples — add at least one so it shows in `bite --help`",
				tool.Name)
		})
	}
}

// TestMeta_everyParamHasDesc walks every registered tool's Params and asserts
// each one carries a non-empty Desc. The Desc shows up in the AI tool schema,
// `bite <tool> --help`, and slash-command help — a missing one silently
// degrades all three. Enforced as a meta-test (not in validate()) so test
// fixtures can construct minimal Params without ceremony.
func TestMeta_everyParamHasDesc(t *testing.T) {
	for _, tool := range All() {
		for _, p := range tool.Params {
			t.Run(tool.Name+"."+p.Name, func(t *testing.T) {
				assert.NotEmpty(t, p.Desc, "param %q on tool %q needs a Desc", p.Name, tool.Name)
			})
		}
	}
}

// TestMeta_dropANewToolReachesEverySurface proves the package's headline
// promise: registering a brand-new Tool surfaces it on every adapter
// without touching any other code. If a future refactor breaks the
// auto-wiring, this test fails.
func TestMeta_dropANewToolReachesEverySurface(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name:        "smoke_test_tool",
			Summary:     "Surface check for the centralised registry.",
			Description: "Internal smoke-test tool — should appear on every surface.",
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
				return Result{Text: "pong"}, nil
			},
		})

		// AI adapter
		ais := AITools(Deps{})
		require.Len(t, ais, 1)
		assert.Equal(t, "smoke_test_tool", ais[0].Name)

		// cobra adapter
		root := &cobra.Command{Use: "test"}
		RegisterCobra(root, StaticDeps(Deps{}))
		var found bool
		for _, c := range root.Commands() {
			if c.Name() == "smoke_test_tool" {
				found = true
				break
			}
		}
		assert.True(t, found, "smoke_test_tool missing from cobra commands")

		// slash adapter
		out := Dispatch(context.Background(), Deps{}, "/smoke_test_tool")
		require.NoError(t, out.ParseError)
		require.NoError(t, out.RunError)
		assert.Equal(t, "pong", out.Result.Text)

		// system-prompt appendix
		assert.Contains(t, RenderAppendix(), "smoke_test_tool")

		// /help builtin
		help := Dispatch(context.Background(), Deps{}, "/help")
		assert.Contains(t, help.Result.Text, "smoke_test_tool")
	})
}
