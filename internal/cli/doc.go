// Package cli wires the cobra command tree.
//
// Every domain command — including chat — is a Tool registered in
// [internal/tools]. `tools.RegisterCobra` mounts them on the root command
// automatically. This package owns only the rootCmd itself plus Execute.
//
// If you find yourself adding a second file here that defines a cobra
// subcommand, it probably belongs in internal/tools/ as a registered Tool.
// Use Tool.SkipAI / Tool.SkipSlash if the command is cobra-only.
package cli
