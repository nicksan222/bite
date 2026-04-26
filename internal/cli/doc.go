// Package cli wires the cobra command tree.
//
// Domain commands live in [internal/tools] — drop a file there and
// `tools.RegisterCobra` mounts it on the root command automatically. The
// matching [internal/tools] helpers (LoadConfig, OpenStore, OpenAIClient,
// CobraDepsProvider) supply runtime wiring so this package stays a thin
// composition layer.
//
// This package only owns the things that aren't ordinary tools:
//
//   - root.go  — rootCmd + Execute (calls tools.RegisterCobra)
//   - chat.go  — the interactive TUI command (cannot fit the Tool shape
//     because it spawns a long-lived bubbletea program)
//
// If you find yourself adding a third file here, it probably belongs in
// internal/tools/ instead.
package cli
