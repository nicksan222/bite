package tools

// Args carries normalised parameter values. The same shape is produced by
// all three surface parsers (AI JSON, cobra flags, slash key=value), so
// every Tool.Run consumes the same Args type regardless of where the call
// came from.
//
// Use Has to distinguish "absent" from "zero" — important for set_goals,
// where omitting a field preserves the current value but explicitly passing
// 0 clears it.
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

// NewArgsForTool wraps a raw map and fills in any Param.Default for params
// the user did not supply. Adapters use this so a Tool.Run can rely on
// args.Int/Float/etc. returning the declared Default — no per-tool
// `if v == 0 { v = N }` boilerplate.
//
// Params *without* a Default are left absent (Has returns false), preserving
// the absent-vs-zero distinction set_goals depends on.
func NewArgsForTool(t Tool, raw map[string]any) Args {
	if raw == nil {
		raw = map[string]any{}
	}
	for _, p := range t.Params {
		if p.Default == nil {
			continue
		}
		if _, supplied := raw[p.Name]; supplied {
			continue
		}
		raw[p.Name] = p.Default
	}
	return Args{raw: raw}
}

// Has reports whether name was supplied.
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

// Int returns the int64 at name. Accepts int/int64/float64/float32 because
// JSON numbers arrive as float64 from the AI adapter while cobra returns
// int64. Returns 0 if absent or wrong type.
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
