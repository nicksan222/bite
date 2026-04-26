// Package cli wires the cobra command tree.
//
// Domain commands live in [internal/tools] — drop a file there and
// `tools.RegisterCobra` mounts it on the root command automatically.
// Runtime wiring (config, store, AI client, chat-launch flow) also lives
// in tools, so this package stays a thin composition layer.
//
// This package only owns the things that aren't ordinary tools:
//
//   - root.go  — rootCmd + Execute (calls tools.RegisterCobra)
//   - chat.go  — cobra wrapper that calls tools.RunChatTUI; here only
//     because cobra needs the cmd registered with rootCmd at init time
//
// If you find yourself adding a third file here, it probably belongs in
// internal/tools/ instead.
package cli
