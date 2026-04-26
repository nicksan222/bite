// Package db is bite's persistence layer.
//
// # Layout
//
//   - db/migrations/ — goose-format SQL migrations (numbered, _up/_down sections).
//   - db/queries/    — sqlc query files (named with `-- name: Foo :one|many|exec`).
//   - db/sqlc/       — sqlc-generated Go code (do not edit by hand).
//   - db/store.go    — the high-level Store façade callers use.
//   - db/migrate.go  — embedded migration runner.
//
// # Adding things
//
//   - New table → new migration in db/migrations/ + queries in db/queries/,
//     then `make sqlc`.
//   - New query → append to the matching db/queries/<entity>.sql file, then
//     `make sqlc`.
//   - New high-level operation → add a method to *db.Store that wraps one or
//     more sqlc calls (transactions, joins-of-joins, business logic).
//
// Callers should depend on *db.Store, not on db/sqlc directly, so the storage
// boundary stays narrow.
package db
