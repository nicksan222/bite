// Package tools is bite's centralised tool registry.
//
// Each domain action (logging a meal, viewing goals, listing meals, …) is
// defined exactly once as a Tool value registered from a package-level init().
// Adapters expose the registry to every surface of the application:
//
//   - AITools(deps)    — Claude tool spec for ai.WithTools
//   - RegisterCobra    — auto-generated cobra subcommands
//   - SlashHandlers    — TUI slash commands inside bite chat
//   - RenderAppendix   — "Available tools" block appended to the system prompt
//
// To add a tool: drop a file in this package with an init() that calls
// Register, plus a focused unit test that calls Run with an in-memory Deps.
// No edits to chat wiring, cobra wiring, TUI wiring, or the system prompt
// are required.
package tools
