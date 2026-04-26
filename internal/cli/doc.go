// Package cli wires together bite's command tree.
//
// # Adding a command
//
//  1. Create cli/<name>.go.
//  2. Build a *cobra.Command and call rootCmd.AddCommand(cmd) from init().
//  3. Use the shared helpers in root.go for config, store, and AI client setup.
//
// The root command launches the chat TUI when invoked with no subcommand,
// making `bite` the natural entry point for a multi-turn conversation.
//
// # Shared helpers
//
//   - loadConfig()      — loads Config from env / .env
//   - openStore()       — opens (and migrates) the SQLite database
//   - openAIClient()    — creates an *ai.Client from Config
//   - withStore()       — convenience wrapper: load config + open store + call fn
package cli
