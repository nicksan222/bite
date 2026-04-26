// Package routes owns every HTTP handler, template, and static asset the
// dashboard serves. Register wires them onto a *fiber.App; helpers
// shared between handlers (JSON error envelopes, template rendering,
// query/form merging) live alongside in helpers.go.
//
// One file per route for the simple endpoints (list_tools.go,
// invoke_tool.go, htmx_tool.go, page_*.go, static.go). The chat
// surface is bigger and splits by concern — the two endpoints share
// state, a template, and an SSE encoding layer:
//
//   - chat_stream.go     — the two HTTP handlers (POST /api/chat, GET stream)
//   - chat_session.go    — cookie-keyed session store
//   - chat_turn.go       — per-turn POST→SSE handoff stash
//   - chat_turn_view.go  — chat-turn HTML template + renderer
//   - chat_sse.go        — SSE encoding primitives shared by every path
//
// Types the caller hands in (Deps, Result, ToolMeta, …) live in
// types.go. The parent internal/web package re-exports them via type
// aliases so external callers never need to know the package was split.
package routes
