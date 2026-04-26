package tools

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// DepsProvider supplies a Deps for one cobra invocation. Returning a non-nil
// cleanup is optional; it is run after the tool's Run completes (success or
// failure). Errors from the provider abort the command before Run is called.
type DepsProvider func(ctx context.Context) (Deps, func(), error)

// StaticDeps returns a DepsProvider that always returns d (no cleanup).
// Useful in tests.
func StaticDeps(d Deps) DepsProvider {
	return func(_ context.Context) (Deps, func(), error) { return d, nil, nil }
}

// RegisterCobra adds one subcommand per registered tool to root. The
// DepsProvider is called once per command invocation, so opening the DB or
// AI client is deferred until a tool actually runs (`bite --help` is free).
//
// If root.Example is empty, RegisterCobra also fills it in from the
// Examples on registered tools — adding a new tool with an Example makes
// it discoverable in `bite --help` without editing root.go.
func RegisterCobra(root *cobra.Command, provide DepsProvider) {
	for _, t := range All() {
		root.AddCommand(toCobraCommand(t, provide))
	}
	if root.Example == "" {
		root.Example = renderExamples()
	}
}

// SetDefault wires root.RunE to the named tool's cobra RunE so `bite` (no
// subcommand) behaves identically to `bite <name>`. Call after RegisterCobra.
// No-op if the tool isn't registered, so callers don't need an existence
// check — the meta-tests already guarantee every registered Tool has a
// cobra subcommand.
func SetDefault(root *cobra.Command, name string) {
	cmd, _, err := root.Find([]string{name})
	if err != nil || cmd == nil || cmd.Name() != name {
		return
	}
	root.RunE = cmd.RunE
}

// renderExamples gathers Example rows from every registered tool and lays
// them out as a padded "  cmd<spaces># desc" block matching cobra's default
// example style. Used for the rootCmd's example block.
func renderExamples() string {
	var rows []Example
	for _, t := range All() {
		rows = append(rows, t.Examples...)
	}
	return renderExamplesFor(rows)
}

// renderExamplesFor lays out a specific Example slice (one tool's, or all
// tools' merged). Returns "" when rows is empty so cobra omits the section.
func renderExamplesFor(rows []Example) string {
	if len(rows) == 0 {
		return ""
	}
	width := 0
	for _, r := range rows {
		if l := len(r.Cmd); l > width {
			width = l
		}
	}
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-*s  # %s\n", width, r.Cmd, r.Desc)
	}
	return strings.TrimRight(b.String(), "\n")
}

func toCobraCommand(t Tool, provide DepsProvider) *cobra.Command {
	use, requiredPos, totalPos := buildUse(t)
	cmd := &cobra.Command{
		Use:     use,
		Short:   t.Summary,
		Long:    t.Long(),
		Example: renderExamplesFor(t.Examples),
		// Pick the cobra Args validator that produces the clearest error
		// message for each positional shape:
		//   - ExactArgs when every positional is required (most common),
		//   - MaximumNArgs when all are optional,
		//   - RangeArgs only for the genuinely-mixed case.
		Args: positionalArgsValidator(requiredPos, totalPos),
	}

	flagSetters := bindFlags(cmd, t.Params)
	cmd.RunE = func(c *cobra.Command, posArgs []string) error {
		raw := map[string]any{}
		if err := absorbPositionals(t, posArgs, raw); err != nil {
			return err
		}
		flagSetters(c, raw)
		deps, cleanup, err := provide(c.Context())
		if err != nil {
			return err
		}
		if cleanup != nil {
			defer cleanup()
		}
		// Cobra is the human-facing surface, so wire StreamWriter for tools
		// that stream incrementally (e.g. ask). countingWriter tracks whether
		// the tool actually wrote anything; if it did, we skip reprinting
		// Result.Text so the user doesn't see it twice.
		cw := &countingWriter{w: c.OutOrStdout()}
		deps.StreamWriter = cw
		res, err := t.Run(c.Context(), deps, NewArgsForTool(t, raw))
		if err != nil {
			return err
		}
		if cw.n > 0 {
			res.Text = ""
		}
		renderForCobra(c.OutOrStdout(), res)
		return nil
	}
	return cmd
}

// positionalArgsValidator picks the friendliest cobra positional-arg
// validator for the given (required, total) shape.
func positionalArgsValidator(required, total int) cobra.PositionalArgs {
	switch required {
	case total:
		return cobra.ExactArgs(required)
	case 0:
		return cobra.MaximumNArgs(total)
	default:
		return cobra.RangeArgs(required, total)
	}
}

// buildUse renders the `Use` string for the cobra command, including
// positional placeholders. Returns (use, required-positional-count, total-positional-count).
func buildUse(t Tool) (string, int, int) {
	sig, required, total := positionalSignature(t)
	use := t.Name
	if sig != "" {
		use += " " + sig
	}
	return use, required, total
}

// positionalSignature returns just the "<a> [b]" placeholder string for a
// tool's positionals, plus the (required, total) counts. Shared with the
// slash help renderer so both surfaces show the same shape.
func positionalSignature(t Tool) (string, int, int) {
	var b strings.Builder
	required, total := 0, 0
	for _, p := range t.Params {
		if !p.Positional {
			continue
		}
		total++
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		if p.Required {
			required++
			fmt.Fprintf(&b, "<%s>", p.Name)
		} else {
			fmt.Fprintf(&b, "[%s]", p.Name)
		}
	}
	return b.String(), required, total
}

// bindFlags registers a typed flag for each non-positional Param and returns
// a setter that, given the command and a map, fills the map from the flags
// the user actually changed.
func bindFlags(cmd *cobra.Command, params []Param) func(*cobra.Command, map[string]any) {
	type binding struct {
		param Param
		flag  string
	}
	var bindings []binding

	for _, p := range params {
		if p.Positional {
			continue
		}
		flag := flagName(p)
		bindings = append(bindings, binding{p, flag})

		switch p.Type {
		case ParamString:
			def, _ := p.Default.(string)
			cmd.Flags().String(flag, def, p.Desc)
		case ParamInt:
			var def int64
			switch v := p.Default.(type) {
			case int:
				def = int64(v)
			case int64:
				def = v
			}
			cmd.Flags().Int64(flag, def, p.Desc)
		case ParamFloat:
			def, _ := p.Default.(float64)
			cmd.Flags().Float64(flag, def, p.Desc)
		case ParamBool:
			def, _ := p.Default.(bool)
			cmd.Flags().Bool(flag, def, p.Desc)
		case ParamStringList:
			def, _ := p.Default.([]string)
			cmd.Flags().StringArray(flag, def, p.Desc)
		}
		if p.Required {
			_ = cmd.MarkFlagRequired(flag)
		}
	}

	return func(c *cobra.Command, raw map[string]any) {
		for _, b := range bindings {
			if !c.Flags().Changed(b.flag) {
				continue
			}
			switch b.param.Type {
			case ParamString:
				v, _ := c.Flags().GetString(b.flag)
				raw[b.param.Name] = v
			case ParamInt:
				v, _ := c.Flags().GetInt64(b.flag)
				raw[b.param.Name] = v
			case ParamFloat:
				v, _ := c.Flags().GetFloat64(b.flag)
				raw[b.param.Name] = v
			case ParamBool:
				v, _ := c.Flags().GetBool(b.flag)
				raw[b.param.Name] = v
			case ParamStringList:
				v, _ := c.Flags().GetStringArray(b.flag)
				raw[b.param.Name] = v
			}
		}
	}
}

// absorbPositionals reads the cobra positional slice into raw, parsing each
// per the corresponding Positional param's type.
func absorbPositionals(t Tool, posArgs []string, raw map[string]any) error {
	i := 0
	for _, p := range t.Params {
		if !p.Positional {
			continue
		}
		if i >= len(posArgs) {
			break
		}
		v, err := parseString(p.Type, posArgs[i])
		if err != nil {
			return fmt.Errorf("argument %q: %w", p.Name, err)
		}
		raw[p.Name] = v
		i++
	}
	return nil
}

// parseString converts a CLI/slash positional string into the right Go type.
func parseString(t ParamType, s string) (any, error) {
	switch t {
	case ParamString:
		return s, nil
	case ParamInt:
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("expected integer, got %q", s)
		}
		return v, nil
	case ParamFloat:
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("expected number, got %q", s)
		}
		return v, nil
	case ParamBool:
		v, err := strconv.ParseBool(s)
		if err != nil {
			return nil, fmt.Errorf("expected bool, got %q", s)
		}
		return v, nil
	case ParamStringList:
		// Single positional string is treated as a one-element list.
		return []string{s}, nil
	}
	return nil, fmt.Errorf("unknown param type")
}

// countingWriter is a tiny io.Writer that records how many bytes were written.
// Used by the cobra adapter to detect whether a tool streamed output.
type countingWriter struct {
	w io.Writer
	n int
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += n
	return n, err
}

// renderForCobra writes a Result to out: Text first, then Table as tabwriter.
func renderForCobra(out io.Writer, r Result) {
	if r.Text != "" {
		fmt.Fprintln(out, r.Text)
	}
	if r.Table == nil {
		return
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if len(r.Table.Headers) > 0 {
		fmt.Fprintln(tw, strings.Join(r.Table.Headers, "\t"))
	}
	for _, row := range r.Table.Rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	if len(r.Table.Footer) > 0 {
		fmt.Fprintln(tw, strings.Join(r.Table.Footer, "\t"))
	}
	_ = tw.Flush()
}
