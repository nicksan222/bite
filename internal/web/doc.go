// Package web is bite's HTTP surface — a Fiber server that exposes the
// same tools-first capability the CLI/TUI/AI use, over HTTP. Pages are
// rendered server-side as Go templates and driven by HTMX; the only JS
// is a small SSE consumer for the chat page (static/chat.js). No JS
// build step, no node_modules — everything ships in the Go binary.
//
// Routes (one file per route under web/routes/) consume a Deps value
// containing closures that bridge to the tool registry. The package
// re-exports those types as aliases so external callers — primarily
// internal/tools/web.go — don't need to know the package was split.
//
// Surfaces:
//
//   - Pages  — GET /, /chat, /meals, /tools (HTMX-rendered HTML)
//   - JSON   — GET /api/tools, POST /api/tools/:name
//   - HTMX   — GET|POST /htmx/tool/:name (HTML fragment for hx-swap)
//   - Chat   — POST /api/chat (SSE-streamed, tool-calls bound via ai.WithTools)
//   - Static — GET /static/* (CSS, JS, embedded)
package web
