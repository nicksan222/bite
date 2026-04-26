package tools

import (
	"context"
	"fmt"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/web"
)

func init() {
	Register(Tool{
		Name:    "web",
		Summary: "Launch the web dashboard (Fiber + HTMX).",
		Description: `Launch bite's web dashboard.

Starts a local Fiber server that exposes every registered tool over JSON
(/api/tools, /api/tools/:name) and HTMX fragments (/htmx/tool/:name), plus
a streaming chat endpoint (/api/chat) backed by the same ai.Streamer the
TUI uses. Pages are server-rendered Go templates driven by HTMX — no JS
build step, the binary is fully self-contained.

The tools registry is the single source of truth: anything reachable via
"bite <tool>" or /<tool> in the TUI is reachable here too, no per-route
boilerplate.`,
		// Launcher tool: as an AI tool it would be recursive (the model is
		// already inside an agent loop), and as /web it would race with the
		// TUI process the chat is running in. Skip both.
		SkipAI:    true,
		SkipSlash: true,
		Examples: []Example{
			{Cmd: "bite web", Desc: "serve the dashboard on http://127.0.0.1:8787"},
			{Cmd: "bite web --port 9000", Desc: "use a different port"},
			{Cmd: "bite web --host 0.0.0.0", Desc: "expose on all interfaces (LAN)"},
		},
		Params: []Param{
			{Name: "host", Type: ParamString, Default: "127.0.0.1",
				Desc: "Address to bind. Default 127.0.0.1; pass 0.0.0.0 to expose on the LAN."},
			{Name: "port", Type: ParamInt, Default: int64(8787),
				Desc: "Port to listen on."},
		},
		Run: runWeb,
	})
}

// runWeb wires the live tool registry into web.Deps and blocks until the
// server exits. Like runChat, this is the single source of truth for the
// surface so any future entry point can reuse it without re-binding the
// registry.
//
// Deliberately skips RequireAI: the dashboard is useful for non-AI tools
// (meals_today, get_goals, …) even without ANTHROPIC_API_KEY. The /api/chat
// handler enforces the key when an actual chat request lands.
func runWeb(ctx context.Context, deps Deps, args Args) (Result, error) {
	cfg := web.Config{
		Host: args.String("host"),
		Port: int(args.Int("port")),
	}

	wd := web.Deps{
		AI:         deps.AI,
		StreamOpts: func() []ai.StreamOption { return ChatStreamOptions(deps) },
		ListTools:  func() []web.ToolMeta { return webToolList() },
		InvokeTool: func(ctx context.Context, name string, raw map[string]any) (web.Result, error) {
			return invokeRegisteredTool(ctx, deps, name, raw)
		},
	}

	srv := web.New(wd)
	// StreamWriter is wired by the cobra adapter to stdout, so this
	// prints the URL once at boot before Listen blocks. AI/slash callers
	// leave StreamWriter nil; staying silent there is the right default.
	if w := deps.StreamWriter; w != nil {
		fmt.Fprintf(w, "bite web listening on http://%s\n", cfg.Addr())
	}
	return Result{}, srv.Listen(ctx, cfg)
}

// webToolList renders every registry tool as web.ToolMeta. SkipAI tools
// are excluded so the HTTP surface can't accidentally invoke launchers
// (chat, web) — same gate the model sees.
func webToolList() []web.ToolMeta {
	all := All()
	out := make([]web.ToolMeta, 0, len(all))
	for _, t := range all {
		if t.SkipAI {
			continue
		}
		params := make([]web.ParamMeta, 0, len(t.Params))
		for _, p := range t.Params {
			params = append(params, web.ParamMeta{
				Name:       p.Name,
				Type:       paramTypeName(p.Type),
				Desc:       p.Desc,
				Required:   p.Required,
				Positional: p.Positional,
			})
		}
		out = append(out, web.ToolMeta{
			Name:        t.Name,
			Summary:     t.Summary,
			Description: t.Long(),
			Params:      params,
		})
	}
	return out
}

// invokeRegisteredTool is the bridge HTTP handlers call to run a tool by
// name. SkipAI tools are NOT reachable here, mirroring webToolList — the
// HTTP surface sees the same world the model does.
//
// Coerces string-shaped values (HTMX form posts arrive as strings) into
// each Param's declared type, so tools can rely on args.Float/Int/Bool
// the same way they do under cobra and the slash dispatcher.
func invokeRegisteredTool(ctx context.Context, deps Deps, name string, raw map[string]any) (web.Result, error) {
	t, ok := Get(name)
	if !ok || t.SkipAI {
		return web.Result{}, web.NotFoundError{Name: name}
	}
	coerced, err := coerceWebArgs(t, raw)
	if err != nil {
		return web.Result{}, err
	}
	res, err := t.Run(ctx, deps, NewArgsForTool(t, coerced))
	if err != nil {
		return web.Result{}, err
	}
	return toWebResult(res), nil
}

// coerceWebArgs walks raw and parses string values into the Param's
// declared Go type. JSON-shaped values (float64, bool, …) pass through
// unchanged. Unknown keys also pass through — tools ignore them, but
// rejecting would break forward-compatibility for HTMX forms that
// include incidental fields.
func coerceWebArgs(t Tool, raw map[string]any) (map[string]any, error) {
	if len(raw) == 0 {
		return raw, nil
	}
	byName := make(map[string]Param, len(t.Params))
	for _, p := range t.Params {
		byName[p.Name] = p
	}
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		p, known := byName[k]
		if !known {
			out[k] = v
			continue
		}
		if s, ok := v.(string); ok && p.Type != ParamString {
			parsed, err := parseString(p.Type, s)
			if err != nil {
				return nil, fmt.Errorf("argument %q: %w", p.Name, err)
			}
			out[k] = parsed
			continue
		}
		out[k] = v
	}
	return out, nil
}

func toWebResult(r Result) web.Result {
	out := web.Result{Text: r.Text}
	if r.Table != nil {
		out.Table = &web.Table{
			Headers: r.Table.Headers,
			Rows:    r.Table.Rows,
			Footer:  r.Table.Footer,
		}
	}
	return out
}

// paramTypeName maps the registry's enum to the wire string. Lives here
// (not in web/) so the web package stays unaware of the registry's type.
func paramTypeName(t ParamType) string {
	switch t {
	case ParamString:
		return "string"
	case ParamInt:
		return "int"
	case ParamFloat:
		return "float"
	case ParamBool:
		return "bool"
	case ParamStringList:
		return "string_list"
	}
	return "unknown"
}
