// Package cli wires the cobra command tree.
//
// Most domain commands live in [internal/tools] — drop a file there and
// `tools.RegisterCobra` mounts it on the root command automatically.
//
// This package only owns the things that aren't ordinary tools:
//
//   - root.go  — rootCmd + Execute + lazy Deps wiring
//   - chat.go  — the interactive TUI command (cannot fit the Tool shape
//     because it spawns a long-lived bubbletea program)
//
// If you find yourself adding a third file here, it probably belongs in
// internal/tools/ instead.
package cli
