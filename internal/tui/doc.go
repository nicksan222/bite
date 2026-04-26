// Package tui owns bite's terminal UIs (Bubbletea programs).
//
// # Layout
//
//   - tui/chat.go    — interactive multi-turn chat program (the default UI).
//   - tui/styles.go  — lipgloss styles shared across screens.
//
// # Adding a new screen
//
//  1. Create `tui/<screen>.go` with a private model + public constructor that
//     returns a *tea.Program (e.g. `tui.NewWizard(...)`).
//  2. Wire it from a cli command (one file per command in cli/).
//  3. Reuse styles.go for visual consistency.
//
// The TUI talks to bite via small interfaces (e.g. tui.Persister) so it
// doesn't grow knowledge of the database or AI framework directly.
package tui
