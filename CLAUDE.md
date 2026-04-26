# CLAUDE.md

Read this before touching anything. It's the contract for working in `bite`.

## What is bite?

**bite is a terminal-first, AI-powered nutritionist.**

You chat with it like a coach in your terminal. It tracks what you eat,
estimates calories and macros, remembers your goals, and answers questions
about food, recipes, and habits — all from a fast, keyboard-driven TUI that
stores your history locally in SQLite.

The MVP is the chat experience (already wired). Domain features layer on top
of that foundation:

- log meals from natural language ("had 200g of pasta with pesto")
- daily / weekly calorie + macro summaries
- goal tracking (target weight, deficit / surplus, protein floor)
- food database lookups (USDA / OpenFoodFacts)
- nudges and streaks

These are added by registering **agents**, **tools**, **commands**, and **DB
tables** — see "Where things go" below.

---

## Where things go

bite is structured so every kind of change has exactly one obvious place.

```
cmd/bite/main.go              # binary entry — keep it tiny
internal/
  ai/                         # AI layer (Claude via eino)
    client.go  claude.go      # streaming Client + provider factory
    messages.go               # Message + Attachment
    analyze.go                # AnalyzeMeal: any media → MealAnalysis
  media/                      # media plumbing (NOT AI-specific)
    kind.go                   # extension → image/audio/video
    audio.go                  # OpenAI Whisper transcription
    video.go                  # ffmpeg keyframe extraction (+ CheckFFmpeg)
  tools/                      # CENTRAL REGISTRY — one file per domain action / check
    registry.go  tool.go      # Tool definition + Register/All/MustGet
    check.go                  # Check definition + RegisterCheck/Checks (doctor)
    ai_adapter.go             # AITools(deps) → []ai.Tool for chat
    cobra_adapter.go          # RegisterCobra(root, provider) — auto subcommands
    slash_adapter.go          # Dispatch(ctx, deps, "/cmd …") for TUI
    systemprompt.go           # RenderAppendix() — auto persona injection
    log_meal.go  meals_today.go  ask.go  doctor.go  …  # one tool per file
    checks_config.go  checks_db.go  checks_ping.go  …  # one Check per concern
  cli/                        # cobra root + thin shims that delegate to tools/
  config/                     # env + .env config (caarlos0/env)
  db/                         # persistence
    migrations/  queries/     # SQL files (sqlc + goose-format)
    sqlc/                     # GENERATED — do not edit
    store.go  migrate.go      # Store façade + embedded migration runner
  tui/                        # bubbletea programs (slash handlers come from tools.NewSlashHandler)
  web/                        # Fiber HTTP surface — HTMX-driven dashboard
    server.go  aliases.go     # New / Listen, type aliases re-exported from routes/
    routes/                   # one file per endpoint
      register.go             # Register(app, deps) — wires every route
      list_tools.go  invoke_tool.go  chat_stream.go  htmx_tool.go
      page_dashboard.go  page_chat.go  page_meals.go  page_tools.go  static.go
      helpers.go  types.go  embed.go
      views/  static/         # HTML templates, CSS, vendored htmx.min.js
```

| Adding… | Drop a file in… | Ritual |
|---|---|---|
| **Domain action / chat tool / slash command / CLI subcommand** | `internal/tools/<name>.go` (+ `<name>_test.go`) | `tools.Register(...)` in `init()` — auto-wires AI tool spec, cobra command, slash handler, system-prompt entry |
| **Doctor health check** | `internal/tools/checks_<concern>.go` | `tools.RegisterCheck(...)` in `init()` — auto-extends `bite doctor` and `bite doctor --help` |
| TUI screen | `internal/tui/<name>.go` (program) + register a launcher Tool in `internal/tools/<name>.go` (use `SkipAI`/`SkipSlash` if cobra-only, like chat) | the cobra-adapter passes `Deps` into your tool's `Run`; build `tui.New(...).Run()` from there |
| Web page / endpoint | one file per route in `internal/web/routes/<name>.go` (+ `<name>.html` template if it's a page) | one line in `internal/web/routes/register.go`; `bite web` already exposes the registry over HTTP, so per-tool work is rarely needed |
| DB table | `internal/db/migrations/000N_*.sql` + `internal/db/queries/<entity>.sql` | `make sqlc` |
| Store method | method on `*db.Store` in `internal/db/store.go` | wrap one or more sqlc calls |
| Config knob | one struct field with `env:"…"` tags in `internal/config/config.go` | nothing else |
| AI capability | function in `internal/ai/` (e.g. `analyze.go`) using the `*Client` | — |
| Media handler | file in `internal/media/` (e.g. another transcription provider) | — |

The `internal/cli/` folder owns only the rootCmd struct + `Execute`. The
no-arg `bite` invocation is wired to the `chat` tool by `tools.SetDefault`
— there is no hand-written `RunE` in cli/. Anything that looks like a
subcommand belongs in `internal/tools/`.

**One tool / one command per file.** If you find yourself putting two
`tools.Register` calls or two `cobra.Command`s in one file, split it.

### How the tool registry works

A `Tool` value carries everything needed by every surface:

```go
tools.Register(tools.Tool{
    Name:        "log_meal",                       // canonical AI / cobra / slash name
    Summary:     "Log a meal to the user's diary.",// one-line — cobra Short, prompt list
    Description: "Estimate macros from text…",     // long form — cobra Long, AI tool desc
    Prompt:      "Call log_meal whenever the user…",// auto-injected into chat persona
    Params: []tools.Param{
        {Name: "title", Type: tools.ParamString, Required: true, Positional: true},
        {Name: "kcal",  Type: tools.ParamFloat},
    },
    Run: func(ctx context.Context, deps tools.Deps, args tools.Args) (tools.Result, error) {
        // ... pure-ish business logic
    },
})
```

Adapters consume the registry:

- `tools.AITools(deps)` → `[]ai.Tool` for `ai.WithTools` (called by the chat tool's `Run`)
- `tools.RegisterCobra(rootCmd, provider)` → cobra subcommands (called by `cli/root.go`)
- `tools.SetDefault(rootCmd, "chat")` → wires the chat tool as `bite`'s no-arg RunE
- `tools.NewSlashHandler(deps)` → TUI slash dispatcher (called by the chat tool's `Run`)
- `tools.BuildSystemPrompt(custom)` → assembles persona + appendix (called by `tools.OpenAIClient`)
- `tools.Checks()` → doctor's check list (`bite doctor` and `bite doctor --help` enumerate the registry)

Wiring helpers also live next to the registry — `LoadConfig`,
`OpenStore`, `OpenAIClient`, `CobraDepsProvider`, `PrepareSession`,
`NewChatPersister`. Adding a new entry point should reuse them rather
than rebuilding the wiring in `cli/`.

Per-tool tests call `Run` directly with an in-memory `Deps`. No cobra,
no `tea.Model`, no AI mock plumbing required. Per-check tests are even
simpler — call `Run(ctx)` directly.

### Optional Tool fields you may need

- `Prompt` — natural-language guidance for the model on *when* to call this
  tool. Auto-injected into the system prompt. Falls back to `Description` if empty.
- `DescribeDynamic func() string` — return the long-form help text computed
  at registration time, after every `init()` has run. Use this when the help
  must reflect current registry state (e.g. `doctor` lists every Check).
- `Examples []Example` — `bite <tool> --help` and `bite --help` both build
  their example blocks from this. Cmd must start with `bite <tool-name>`
  (validated at registration); Desc must be non-empty.
- `Param.Default` — the canonical fallback value when the user omits the
  param. Adapters inject it via `NewArgsForTool`, so a tool's `Run` can call
  `args.Int("limit")` and trust the declared default — no per-tool
  `if v == 0 { v = N }` boilerplate. Param.Default is mutually exclusive
  with `Required` (a default makes the param optional).
- `SkipAI` / `SkipSlash` — opt out of the AI-tool-spec / slash dispatcher.
  Set both for cobra-only commands like `chat` (the launcher itself can't
  meaningfully be called by the model or recursively from inside a TUI).
  The cobra adapter still mounts the subcommand. Use `tools.SetDefault` to
  wire a tool as the rootCmd's no-arg behavior.
- `Deps.StreamWriter` — write progressive output here for tools that stream
  (e.g. `ask`). The cobra adapter wires it to stdout; AI/slash leave it nil.
- `Deps.RequireAI()` — call this at the top of any `Run` that absolutely
  needs the model (chat does). Surfaces missing-API-key errors before
  blocking on user input, so `bite chat` with no key fails fast instead of
  opening the TUI and breaking on the first message.

---

## Conventions

### Boundaries
- The TUI never imports `internal/db` or sqlc directly — it talks to a small
  `tui.Persister` interface, satisfied by `tools.ChatPersister`.
- Callers depend on `*db.Store`, not on `internal/db/sqlc`. The store
  re-exports `db.Conversation` and `db.Message` so importers stay clean.
- The AI layer never imports `internal/config` — `tools.OpenAIClient`
  builds an `ai.ClientConfig` from a loaded `config.Config` and passes it in.

### Errors
- Wrap with `fmt.Errorf("doing X: %w", err)` to preserve chains.
- Don't wrap at the top of `main` or in cobra `RunE` — fang formats them.
- For user-actionable failures (missing API key, etc.) put the fix in the
  error message itself, not in surrounding prose.

### Tests
- Test behavior, not implementation. **Hardcoded lists of subcommand names,
  agent names, or tool names are banned** — they rot the moment you add a new
  one. Test the registration *mechanism* instead (e.g. duplicate-panic).
- DB tests run against `:memory:` SQLite — fast, no fixtures.
- Skip mocking the AI provider; small contract tests on message-shape
  conversion are enough at this layer.

### Comments
- Default to no comments. Only write one when *why* is non-obvious — a hidden
  constraint, a workaround for a bug, an invariant that would surprise.
- Don't restate what the code does. Don't reference the current task or PR.
- Package-level docs live in `doc.go` files; type/function docs go inline.

### Reinventing wheels
We use libraries for everything that has one. Audit before hand-rolling:
- env config → `caarlos0/env/v11` + `joho/godotenv`
- XDG paths → `adrg/xdg`
- migrations → `pressly/goose`
- SQLite → `modernc.org/sqlite` (pure Go, no CGO)
- SQL codegen → `sqlc`
- TUI → `charmbracelet/{bubbletea,bubbles,lipgloss,glamour}`
- CLI → `spf13/cobra` + `charmbracelet/fang`
- AI → `cloudwego/eino` + `eino-ext/components/model/{claude,openai,gemini,ollama}`
- text truncation → `mattn/go-runewidth`

If you reach for `os.Getenv` directly, stop — add a field to `config.Config`
with the right tag. (One carve-out: `internal/ai/provider.go` reads each
provider's credential and base-URL env vars itself, since `ProviderSpec`
declares them. That keeps adding a provider a one-file change.)

### Multi-provider AI
bite supports Anthropic (default), OpenAI, Gemini, and Ollama. Each backend
lives in its own file under `internal/ai/` (`claude.go`, `openai.go`,
`gemini.go`, `ollama.go`) and self-registers a `ProviderSpec` from `init()`.
Consumers (`NewClient`, doctor checks, wiring) only see the registry — they
never switch on provider name.

To add a new provider, drop a file next to the others that calls
`ai.RegisterProvider(ProviderSpec{...})` with the env var, default model,
and a `Build` factory. Nothing else needs touching: `bite doctor` picks up
its credential check automatically, and `BITE_PROVIDER=<name>` selects it.

The provider's own `Validate` returns a typed `*MissingCredentialError`
naming its env var, so consumers get user-actionable errors without
knowing the implementation.

### Multimodal
`bite analyze_meal` (estimate without saving) and `bite log_meal_from_media`
(estimate + save) are the two entry points for any combination of images,
audio, video, and text. Pre-processing lives in `internal/media`:

- **Images** → sent directly via the active provider's vision endpoint
  (no preprocessing).
- **Audio** → OpenAI Whisper (`OPENAI_API_KEY` required); transcript is
  folded into the prompt text.
- **Video** → `ffmpeg` keyframe extraction; frames are sent as images.
  ffmpeg is a soft dependency — `bite doctor` warns if missing; videos
  passed without it produce a clear, actionable error.
- **Text** → just text (combined with everything above).

`internal/ai` owns the model call. `internal/media` owns the format
plumbing. They don't import each other beyond `ai/analyze.go` consuming
`media`'s outputs (text + image paths).

---

## Commit style

- **Subject only.** No body, no description, no trailers. One line.
- **Max 72 characters.** Hard rule.
- Imperative mood: `add meal-logging tool`, not `added` or `adds`.
- Prefer one logical change per commit. Refactors stand alone.
- **No AI attribution.** Do not add `Co-Authored-By: Claude`,
  `🤖 Generated with Claude Code`, or any similar trailer / footer.

Examples:
```
add meal-logging tool with macro estimation
fix chat resume losing system prompt
refactor store: collapse migrate.go into store.go
```

## PR style

- Title mirrors the commit subject rule (≤72 chars, imperative).
- Body has a "Summary" section and a "Test plan" checklist.
- Same no-AI-attribution rule as commits — no generated footers.

---

## Day-to-day

```bash
make setup           # one-time: tidy + sqlc generate
make run             # opens chat TUI
make test            # go test -race ./...
make lint            # golangci-lint
make sqlc            # regenerate db/sqlc after editing SQL
make migrate-new name=add_foods   # scaffold a new migration
```

`bite doctor` (or `bite doctor --ping`) is the diagnostic — run it whenever
something feels off before reporting a bug.

## Skills

bite uses the [skills.sh](https://skills.sh) directory. Skills live in
`.agents/skills/` and `.claude/skills` is a symlink to that same directory so
Claude Code and other agents see one source of truth. The lockfile
(`skills-lock.json`) is committed; restore with `npx skills experimental_install`.
