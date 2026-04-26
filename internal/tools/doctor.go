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
		// Both Description and DescribeDynamic must be non-empty for the
		// registry to accept the tool. Description is the static fallback;
		// DescribeDynamic is what cobra actually shows so the help text picks
		// up checks registered in any order.
		Description:     "Run health checks on bite's environment.",
		DescribeDynamic: doctorDescription,
		Prompt: `Use doctor when the user reports something not working, asks "is bite set up?",
or wants to verify the environment after a fresh install.`,
		Examples: []Example{
			{Cmd: "bite doctor --ping", Desc: "verify environment + model reachability"},
		},
		// Params are derived from the gates declared by registered checks.
		// Adding a gated check (e.g. Gate: "verbose") makes --verbose appear
		// on `bite doctor` automatically — no edits to this file required.
		Params: gateParams(),
		Run:    runDoctor,
	})
}

// gateParams collects every distinct Check.Gate from the registered checks
// and returns them as Bool params. Order is stable (registration order via
// Checks()) so help output doesn't flap.
func gateParams() []Param {
	seen := map[string]struct{}{}
	var params []Param
	for _, c := range Checks() {
		if c.Gate == "" {
			continue
		}
		if _, dup := seen[c.Gate]; dup {
			continue
		}
		seen[c.Gate] = struct{}{}
		params = append(params, Param{
			Name: c.Gate, Type: ParamBool,
			Desc: fmt.Sprintf("Run gated check %q.", c.Name),
		})
	}
	return params
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
	fmt.Fprintf(b, "  • %s — %s%s\n", c.Name, c.Desc, suffix)
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
		if !runCheck(ctx, &sb, c) {
			failed++
		}
	}
	if len(softs) > 0 {
		sb.WriteString("\n")
		for _, c := range softs {
			runCheck(ctx, &sb, c)
		}
	}
	if failed > 0 {
		fmt.Fprintf(&sb, "\n%d required check(s) failed.\n", failed)
		return Result{Text: sb.String()}, fmt.Errorf("doctor: %d failed", failed)
	}
	sb.WriteString("\nAll required checks passed.")
	return Result{Text: sb.String()}, nil
}

// runCheck executes one Check and writes a one-line status row. The fail
// glyph derives from c.Severity — Hard ✗, Soft ! — so adding a new severity
// only requires extending Severity.FailGlyph.
func runCheck(ctx context.Context, b *strings.Builder, c Check) bool {
	detail, err := c.Run(ctx)
	if err != nil {
		fmt.Fprintf(b, "  %s %-22s  %v\n", c.Severity.FailGlyph(), c.Name, err)
		return false
	}
	fmt.Fprintf(b, "  ✓ %-22s  %s\n", c.Name, detail)
	return true
}
