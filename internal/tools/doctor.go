package tools

import (
	"context"
	"fmt"
	"strings"
)

func init() {
	Register(Tool{
		Name:    "doctor",
		Summary: "Run health checks on bite's environment.",
		// Description is computed dynamically from the Check registry — adding
		// a new Check automatically extends the help text.
		DescribeDynamic: doctorDescription,
		Description:     doctorDescription(),
		Prompt: `Use doctor when the user reports something not working, asks "is bite set up?",
or wants to verify the environment after a fresh install.`,
		Params: []Param{
			{Name: "ping", Type: ParamBool,
				Desc: "Send a tiny test request to the model."},
		},
		Run: runDoctor,
	})
}

// doctorDescription assembles the full Long help text from the registered
// Check list, so adding a Check shows up in `bite doctor --help` automatically.
func doctorDescription() string {
	var b strings.Builder
	b.WriteString("doctor verifies that bite is ready to run.\n\nChecks (auto-registered):\n")
	var soft []Check
	for _, c := range Checks() {
		if c.Severity == SeveritySoft {
			soft = append(soft, c)
			continue
		}
		describeCheck(&b, c)
	}
	if len(soft) > 0 {
		b.WriteString("\nSoft checks (warnings only):\n")
		for _, c := range soft {
			describeCheck(&b, c)
		}
	}
	return b.String()
}

func describeCheck(b *strings.Builder, c Check) {
	suffix := ""
	if c.Gate != "" {
		suffix = fmt.Sprintf("  (only with --%s)", c.Gate)
	}
	desc := c.Desc
	if desc == "" {
		desc = c.Name
	}
	fmt.Fprintf(b, "  • %s — %s%s\n", c.Name, desc, suffix)
}

func runDoctor(ctx context.Context, _ Deps, args Args) (Result, error) {
	var sb strings.Builder
	failed := 0
	var softs []Check
	for _, c := range Checks() {
		if c.Gate != "" && !args.Bool(c.Gate) {
			continue
		}
		if c.Severity == SeveritySoft {
			softs = append(softs, c)
			continue
		}
		if !runCheck(ctx, &sb, c, "✗") {
			failed++
		}
	}
	if len(softs) > 0 {
		sb.WriteString("\n")
		for _, c := range softs {
			runCheck(ctx, &sb, c, "!")
		}
	}
	if failed > 0 {
		fmt.Fprintf(&sb, "\n%d required check(s) failed.\n", failed)
		return Result{Text: sb.String()}, fmt.Errorf("doctor: %d failed", failed)
	}
	sb.WriteString("\nAll required checks passed.")
	return Result{Text: sb.String()}, nil
}

func runCheck(ctx context.Context, b *strings.Builder, c Check, failGlyph string) bool {
	detail, err := c.Run(ctx)
	if err != nil {
		fmt.Fprintf(b, "  %s %-22s  %v\n", failGlyph, c.Name, err)
		return false
	}
	fmt.Fprintf(b, "  ✓ %-22s  %s\n", c.Name, detail)
	return true
}
