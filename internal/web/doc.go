// Package web is bite's HTTP surface — a Fiber server that exposes the
// same tools-first capability the CLI/TUI/AI use, over HTTP. Every page
// is rendered server-side and driven by htmx; there is zero hand-written
// application JS. htmx and htmx-ext-sse are vendored into static/ for
// offline reproducibility. Tailwind/daisyUI are pulled from
// cdn.jsdelivr.net at page load (the dashboard is local-only by default,
// so the CDN dependency is an acceptable trade for skipping a CSS
// pipeline).
//
// Routes (one file per route under web/routes/) consume a Deps value
// containing closures that bridge to the tool registry. The package
// re-exports those types as aliases so external callers — primarily
// internal/tools/web.go — don't need to know the package was split.
//
// Surfaces:
//
//   - Pages  — GET /, /dashboard, /meals, /tools (server-rendered, htmx-driven)
//   - JSON   — GET /api/tools, POST /api/tools/:name
//   - HTMX   — GET|POST /htmx/tool/:name (HTML fragment for hx-swap)
//   - Chat   — POST /api/chat (returns the dual-bubble HTML scaffold)
//     GET  /api/chat/stream/:id (SSE stream consumed by htmx-ext-sse)
//   - Static — GET /static/* (CSS, JS, embedded)
package web
