// Package routes owns every HTTP handler, template, and static asset the
// dashboard serves. Register wires them onto a *fiber.App; helpers
// shared between handlers (JSON error envelopes, template rendering,
// query/form merging) live alongside in helpers.go.
//
// One file per route — the file map is the route map. Types the caller
// hands in (Deps, Result, ToolMeta, …) live in types.go. The parent
// internal/web package re-exports them via type aliases so external
// callers never need to know the package was split.
package routes
