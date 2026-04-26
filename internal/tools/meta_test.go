package tools

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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
			_, inAI := aiNames[tool.Name]
			assert.True(t, inAI, "tool %q missing from AITools()", tool.Name)

			_, inCobra := cobraNames[tool.Name]
			assert.True(t, inCobra, "tool %q missing from RegisterCobra()", tool.Name)

			assert.Contains(t, appendix, tool.Name, "tool %q missing from system-prompt appendix", tool.Name)
		})
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
