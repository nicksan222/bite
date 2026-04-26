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
//  2. Register the launcher as a Tool in internal/tools (mirroring chat.go)
//     so cobra/AI/slash adapters wire it for free; pass cobra-built Deps
//     into the TUI program. SkipAI/SkipSlash if the launcher is cobra-only.
//  3. Add a thin cobra wrapper in cli/<screen>.go that delegates to the
//     tools launch function.
//  4. Reuse styles.go for visual consistency.
//
// The TUI talks to bite via small interfaces (e.g. tui.Persister) so it
// doesn't grow knowledge of the database or AI framework directly.
package tui
