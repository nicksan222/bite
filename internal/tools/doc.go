// Package tools is bite's centralised registry for domain actions and
// environment health checks.
//
// Each domain action (logging a meal, listing goals, …) is defined exactly
// once as a Tool registered from a package-level init(). Each doctor probe
// is similarly defined as a Check via RegisterCheck. Adapters expose the
// registries to every surface of the application:
//
//   - AITools(deps)         — Claude tool spec for ai.WithTools (skips SkipAI)
//   - ChatStreamOptions     — `ai.WithTools(AITools(deps))` packaged for callers
//   - RegisterCobra         — auto-generated cobra subcommands (one per Tool)
//   - SetDefault            — wires a named tool as rootCmd's no-arg RunE
//   - NewSlashHandler       — TUI slash dispatcher (skips SkipSlash)
//   - RenderAppendix        — "Available tools" block for the system prompt (skips SkipAI)
//   - BuildSystemPrompt     — assembles persona + appendix in one step
//   - Checks                — full Check list iterated by the doctor tool
//   - LoadConfig / OpenStore / OpenAIClient / CobraDepsProvider — wiring helpers
//
// To add a tool: drop a file in this package with an init() that calls
// Register, plus a focused unit test that calls Run with an in-memory Deps.
// To add a doctor check: same, but call RegisterCheck. No edits to cli/, tui/,
// or the system prompt are required — the meta-tests guarantee every Tool
// surfaces in every adapter (or, for SkipAI/SkipSlash tools, that they're
// correctly hidden).
package tools
