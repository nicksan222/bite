// Package tui owns bite's terminal UIs (Bubbletea programs).
//
// # Layout
//
//   - tui/chat.go    — interactive multi-turn chat program (the default UI).
//   - tui/styles.go  — lipgloss styles shared across screens.
//
// # Adding a new screen
//
//  1. Create `tui/<screen>.go` with a private model + public constructor
//     that returns a *tea.Program (e.g. `tui.NewWizard(...)`).
//  2. Register the launcher as a Tool in `internal/tools/<screen>.go`
//     (mirroring chat.go). The cobra adapter mounts it as a subcommand for
//     free. Set SkipAI/SkipSlash if the screen is cobra-only — chat does,
//     because the model can't enter a chat from inside a chat. The Tool's
//     Run receives the cobra-built Deps and constructs the *tea.Program.
//  3. (Optional) `tools.SetDefault(rootCmd, "<name>")` in cli/root.go's
//     Execute if this screen should be the no-arg `bite` behavior.
//  4. Reuse styles.go for visual consistency.
//
// The TUI talks to bite via small interfaces (e.g. tui.Persister) so it
// doesn't grow knowledge of the database or AI framework directly.
package tui
