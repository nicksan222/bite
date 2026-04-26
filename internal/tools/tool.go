package tools

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
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

// Deps is the wiring passed once at startup. Every tool's Run closure receives
// it; tools never reach for globals.
type Deps struct {
	Store        db.Storer
	AI           ai.Streamer
	Now          func() time.Time
	Loc          *time.Location
	OpenAIAPIKey string
	// StreamWriter, when non-nil, receives progressive output from tools that
	// stream (e.g. ask). Cobra wires this to stdout so `bite ask` feels live;
	// AI/slash adapters leave it nil so the model gets the buffered final
	// text via Result.Text.
	StreamWriter io.Writer
}

// NowOrDefault returns d.Now() if set, otherwise time.Now.
func (d Deps) NowOrDefault() time.Time {
	if d.Now != nil {
		return d.Now()
	}
	return time.Now()
}

// LocOrDefault returns d.Loc if set, otherwise time.Local.
func (d Deps) LocOrDefault() *time.Location {
	if d.Loc != nil {
		return d.Loc
	}
	return time.Local
}

// Args carries normalised parameter values. The same shape is produced by all
// three surface parsers (AI JSON, cobra flags, slash key=value).
type Args struct {
	raw map[string]any
}

// NewArgs wraps a raw map. Used by adapters and tests.
func NewArgs(raw map[string]any) Args {
	if raw == nil {
		raw = map[string]any{}
	}
	return Args{raw: raw}
}

// Has reports whether name was supplied (distinguishes absent from zero).
func (a Args) Has(name string) bool {
	_, ok := a.raw[name]
	return ok
}

// String returns the string at name, or "" if absent or wrong type.
func (a Args) String(name string) string {
	v, ok := a.raw[name]
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Int returns the int64 at name. Accepts int/int64/float64 (JSON numbers
// arrive as float64). Returns 0 if absent or wrong type.
func (a Args) Int(name string) int64 {
	v, ok := a.raw[name]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case float32:
		return int64(n)
	}
	return 0
}

// Float returns the float64 at name. Accepts float32/float64/int/int64.
func (a Args) Float(name string) float64 {
	v, ok := a.raw[name]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return 0
}

// Bool returns the bool at name.
func (a Args) Bool(name string) bool {
	v, ok := a.raw[name]
	if !ok {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// StringList returns a []string at name. Accepts []string or []any of strings.
func (a Args) StringList(name string) []string {
	v, ok := a.raw[name]
	if !ok {
		return nil
	}
	switch xs := v.(type) {
	case []string:
		return xs
	case []any:
		out := make([]string, 0, len(xs))
		for _, x := range xs {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
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

// Tool is the single source of truth for a domain action.
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
