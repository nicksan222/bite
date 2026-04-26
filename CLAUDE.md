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
  cli/                        # cobra commands — one file per command
  config/                     # env + .env config (caarlos0/env)
  db/                         # persistence
    migrations/  queries/     # SQL files (sqlc + goose-format)
    sqlc/                     # GENERATED — do not edit
    store.go  migrate.go      # Store façade + embedded migration runner
  tui/                        # bubbletea programs
```

| Adding… | Drop a file in… | Ritual |
|---|---|---|
| CLI command | `internal/cli/<name>.go` | `rootCmd.AddCommand(...)` in `init()` |
| TUI screen | `internal/tui/<name>.go` | export `NewXxx(...) *tea.Program` |
| DB table | `internal/db/migrations/000N_*.sql` + `internal/db/queries/<entity>.sql` | `make sqlc` |
| Store method | method on `*db.Store` in `internal/db/store.go` | wrap one or more sqlc calls |
| Config knob | one struct field with `env:"…"` tags in `internal/config/config.go` | nothing else |
| AI capability | function in `internal/ai/` (e.g. `analyze.go`) using the `*Client` | — |
| Media handler | file in `internal/media/` (e.g. another transcription provider) | — |

**One command per file.** If you find yourself putting two `cobra.Command`s in
one file, split it.

---

## Conventions

### Boundaries
- The TUI never imports `internal/db` or sqlc directly — it talks to a small
  `tui.Persister` interface, satisfied by `*db.Store` adapters in `internal/cli`.
- Callers depend on `*db.Store`, not on `internal/db/sqlc`. The store
  re-exports `db.Conversation` and `db.Message` so importers stay clean.
- The AI layer never imports `internal/config` — `internal/cli/root.go`
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
- AI → `cloudwego/eino` + `eino-ext/components/model/claude`
- text truncation → `mattn/go-runewidth`

If you reach for `os.Getenv` directly, stop — add a field to `config.Config`
with the right tag.

### Claude-only by design
bite uses Claude (Anthropic) and has no abstraction for swapping providers.
The model factory lives in `internal/ai/claude.go`. If a second provider is
ever needed, add a sibling file (e.g. `openai.go`) — don't build a generic
provider system speculatively.

### Multimodal
`bite analyze` is the single entry point for any combination of images,
audio, video, and text. Pre-processing lives in `internal/media`:

- **Images** → sent directly via Claude vision (no preprocessing).
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
