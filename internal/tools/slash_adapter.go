package tools

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// SlashOutcome is the result of dispatching one slash command. ParseError is
// set when the line could not be parsed (unknown command, missing required
// positional, type mismatch). RunError is set when the tool itself returned
// an error. Both are nil on success.
type SlashOutcome struct {
	Result     Result
	ParseError error
	RunError   error
}

// NewSlashHandler returns a function compatible with tui.SlashHandler that
// wraps Dispatch + RenderResultForChat so callers don't have to repeat the
// parse/run/render adapter dance.
//
// Returned errors are the union of parse and run errors — surface them
// inline; on success the string is the chat-rendered result.
func NewSlashHandler(deps Deps) func(ctx context.Context, line string) (string, error) {
	return func(ctx context.Context, line string) (string, error) {
		out := Dispatch(ctx, deps, line)
		if out.ParseError != nil {
			return "", out.ParseError
		}
		if out.RunError != nil {
			return "", out.RunError
		}
		return RenderResultForChat(out.Result), nil
	}
}

// Dispatch parses a slash line, looks up the tool, runs it, and returns the
// outcome. The /help builtin lists every registered tool.
func Dispatch(ctx context.Context, deps Deps, line string) SlashOutcome {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "/") {
		return SlashOutcome{ParseError: fmt.Errorf("not a slash command")}
	}
	name, rest, _ := strings.Cut(line[1:], " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return SlashOutcome{ParseError: fmt.Errorf("empty slash command — type /help for the list")}
	}

	if name == "help" {
		return SlashOutcome{Result: Result{Text: helpText()}}
	}

	tool, ok := Get(name)
	if !ok || tool.SkipSlash {
		// SkipSlash tools (like /chat) deliberately have no slash form. Treat
		// them as unknown so the dispatcher's error message stays consistent.
		return SlashOutcome{ParseError: fmt.Errorf("unknown command: /%s — type /help for the list", name)}
	}

	args, err := parseSlashArgs(tool, rest)
	if err != nil {
		return SlashOutcome{ParseError: err}
	}
	res, err := tool.Run(ctx, deps, args)
	if err != nil {
		return SlashOutcome{RunError: err}
	}
	return SlashOutcome{Result: res}
}

// parseSlashArgs tokenises rest as a mix of positionals and key=value pairs,
// validates required params, and returns Args ready for Run. Param Defaults
// are applied via NewArgsForTool so absent optionals get their declared value.
func parseSlashArgs(t Tool, rest string) (Args, error) {
	tokens, err := tokeniseSlash(rest)
	if err != nil {
		return Args{}, err
	}
	raw := map[string]any{}

	// Split tokens into positionals (until first key=value) and keyed.
	var positionals, keyed []string
	for _, tok := range tokens {
		if isKeyValue(tok) {
			keyed = append(keyed, tok)
			continue
		}
		if len(keyed) > 0 {
			return Args{}, fmt.Errorf("positional %q must come before key=value pairs", tok)
		}
		positionals = append(positionals, tok)
	}

	// Fill positional params in declaration order.
	var positionalParams []Param
	for _, p := range t.Params {
		if p.Positional {
			positionalParams = append(positionalParams, p)
		}
	}
	if len(positionals) > len(positionalParams) {
		return Args{}, fmt.Errorf("too many positional arguments (got %d, want at most %d)", len(positionals), len(positionalParams))
	}
	for i, tok := range positionals {
		p := positionalParams[i]
		v, err := parseString(p.Type, tok)
		if err != nil {
			return Args{}, fmt.Errorf("argument %q: %w", p.Name, err)
		}
		raw[p.Name] = v
	}

	// Fill keyed params.
	byName := map[string]Param{}
	for _, p := range t.Params {
		byName[p.Name] = p
	}
	for _, kv := range keyed {
		key, val, _ := strings.Cut(kv, "=")
		key = strings.TrimSpace(key)
		// Accept both kebab-case (CLI style) and snake_case (canonical).
		key = strings.ReplaceAll(key, "-", "_")
		p, ok := byName[key]
		if !ok {
			return Args{}, fmt.Errorf("unknown parameter %q", key)
		}
		v, err := parseString(p.Type, val)
		if err != nil {
			return Args{}, fmt.Errorf("argument %q: %w", p.Name, err)
		}
		// StringList: append if already present.
		if p.Type == ParamStringList {
			if cur, exists := raw[p.Name]; exists {
				if curList, ok := cur.([]string); ok {
					raw[p.Name] = append(curList, val)
					continue
				}
			}
			raw[p.Name] = []string{val}
			continue
		}
		raw[p.Name] = v
	}

	// Check required params.
	for _, p := range t.Params {
		if !p.Required {
			continue
		}
		if _, ok := raw[p.Name]; !ok {
			return Args{}, fmt.Errorf("missing required argument %q", p.Name)
		}
	}
	return NewArgsForTool(t, raw), nil
}

// isKeyValue reports whether tok looks like a "name=value" pair where the
// name part is a plausible identifier (letters, digits, _ , -).
func isKeyValue(tok string) bool {
	eq := strings.IndexByte(tok, '=')
	if eq <= 0 {
		return false
	}
	name := tok[:eq]
	for i, r := range name {
		switch {
		case unicode.IsLetter(r) || r == '_':
		case unicode.IsDigit(r) && i > 0:
		case r == '-' && i > 0:
		default:
			return false
		}
	}
	return true
}

// tokeniseSlash splits a slash-command rest string into tokens, honouring
// double-quoted spans so values may contain spaces.
func tokeniseSlash(s string) ([]string, error) {
	var tokens []string
	var cur strings.Builder
	inQuote := false
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteByte(c)
		}
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quoted string")
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens, nil
}

// helpText lists every registered tool with its summary. Generated, not
// hand-maintained: a new tool shows up automatically. Positional params are
// included in the displayed signature so the user can see what to type.
func helpText() string {
	all := All()
	if len(all) == 0 {
		return "no slash commands registered"
	}

	type row struct{ left, right string }
	rows := make([]row, 0, len(all)+1)
	width := 0
	for _, t := range all {
		if t.SkipSlash {
			continue
		}
		sig, _, _ := positionalSignature(t)
		left := "/" + t.Name
		if sig != "" {
			left += " " + sig
		}
		if l := len(left); l > width {
			width = l
		}
		rows = append(rows, row{left, t.Summary})
	}
	rows = append(rows, row{"/help", "show this list"})
	if l := len("/help"); l > width {
		width = l
	}

	var b strings.Builder
	b.WriteString("Slash commands:\n")
	for i, r := range rows {
		fmt.Fprintf(&b, "  %-*s  — %s", width, r.left, r.right)
		if i < len(rows)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
