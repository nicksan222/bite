package tools

import (
	"context"
	"fmt"
	"strings"
)

// ParamType identifies the wire type of a tool parameter.
type ParamType int

const (
	ParamString ParamType = iota
	ParamInt
	ParamFloat
	ParamBool
	ParamStringList
)

// Param is one input parameter of a tool. The same Param is used to generate
// the AI JSON-schema, the cobra flag/positional, and the slash-command parser.
type Param struct {
	Name       string
	Type       ParamType
	Desc       string
	Required   bool
	Positional bool
	Default    any
}

// Result is what a tool returns. Text is required; Table is an optional
// structured render that adapters lay out per surface (tabwriter for cobra,
// Markdown table for AI/TUI).
type Result struct {
	Text  string
	Table *Table
}

// Table is a simple tabular result.
type Table struct {
	Headers []string
	Rows    [][]string
	Footer  []string
}

// Tool is the single source of truth for a domain action. Drop a file in
// internal/tools/<name>.go that calls Register from init() and the AI tool
// spec, cobra subcommand, slash handler, and system-prompt entry are all
// wired automatically.
type Tool struct {
	Name        string
	Summary     string
	Description string
	// DescribeDynamic, if non-nil, returns the long-form help text. Called by
	// the cobra adapter at registration time, AFTER every init() has run, so
	// the description can reflect global registry state (e.g. doctor lists
	// every registered Check). When set, takes precedence over Description.
	DescribeDynamic func() string
	// Prompt is an optional system-prompt fragment auto-injected into the
	// chat persona. Use it to teach the model *when* to call this tool. The
	// fragment is written from the model's POV (e.g. "Call this when the user
	// reports eating something — even casually."). Falls back to Description
	// if empty.
	Prompt string
	Params []Param
	Run    func(ctx context.Context, deps Deps, args Args) (Result, error)
}

// Long returns the description used for cobra Long help. Prefers
// DescribeDynamic if set so help reflects current registry state.
func (t Tool) Long() string {
	if t.DescribeDynamic != nil {
		if s := t.DescribeDynamic(); s != "" {
			return s
		}
	}
	return t.Description
}

// validate checks invariants on a Tool definition. Returns a non-nil error
// for any structural problem.
func (t Tool) validate() error {
	if t.Name == "" {
		return fmt.Errorf("tool: empty Name")
	}
	if !isSnakeCase(t.Name) {
		return fmt.Errorf("tool %q: Name must be lower_snake_case", t.Name)
	}
	if t.Summary == "" {
		return fmt.Errorf("tool %q: empty Summary", t.Name)
	}
	if t.Description == "" {
		return fmt.Errorf("tool %q: empty Description", t.Name)
	}
	if t.Run == nil {
		return fmt.Errorf("tool %q: nil Run", t.Name)
	}

	seen := make(map[string]struct{}, len(t.Params))
	sawOptionalPositional := false
	for _, p := range t.Params {
		if p.Name == "" {
			return fmt.Errorf("tool %q: empty Param.Name", t.Name)
		}
		if !isSnakeCase(p.Name) {
			return fmt.Errorf("tool %q: param %q must be lower_snake_case", t.Name, p.Name)
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("tool %q: duplicate param %q", t.Name, p.Name)
		}
		seen[p.Name] = struct{}{}
		if p.Positional {
			if !p.Required {
				sawOptionalPositional = true
			} else if sawOptionalPositional {
				return fmt.Errorf("tool %q: required positional %q must come before optional positionals", t.Name, p.Name)
			}
		}
	}
	return nil
}

// isSnakeCase reports whether s is lower_snake_case (matches AI tool naming
// and standard cobra subcommand convention).
func isSnakeCase(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		case r == '_':
			if i == 0 || i == len(s)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// flagName returns the kebab-cased CLI flag name for a Param.
func flagName(p Param) string { return strings.ReplaceAll(p.Name, "_", "-") }
