package routes

import (
	"context"

	"github.com/nicksan222/bite/internal/ai"
)

// Deps is the wiring Register needs from the caller (the `web` tool's
// Run closure builds it from the live tool registry). Closures are
// invoked per request — the caller decides whether to memoise.
//
// The types live in this subpackage, not the parent web/, so handlers
// can reference them without importing the parent and creating a cycle
// (the parent imports routes to call Register). The parent re-exports
// every type as a name-preserving alias.
type Deps struct {
	// AI is the streaming model client. Required for /api/chat.
	AI ai.Streamer
	// StreamOpts is called per chat request; it must return the
	// ai.WithTools binding so the model can call registered tools the
	// same way it can in the TUI.
	StreamOpts func() []ai.StreamOption
	// ListTools enumerates every tool exposed over HTTP — typically
	// every registered tool minus those marked SkipAI.
	ListTools func() []ToolMeta
	// InvokeTool runs a tool by name with raw map args. Return
	// NotFoundError to produce a 404 instead of a 400.
	InvokeTool func(ctx context.Context, name string, raw map[string]any) (Result, error)
}

// ToolMeta is the wire shape for a tool description. Mirrors the
// relevant subset of tools.Tool without coupling this package to the
// registry.
type ToolMeta struct {
	Name        string      `json:"name"`
	Summary     string      `json:"summary"`
	Description string      `json:"description"`
	Params      []ParamMeta `json:"params"`
}

// ParamMeta describes one tool parameter. Type is the lower-case scalar
// name ("string", "int", "float", "bool", "string_list").
type ParamMeta struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Desc       string `json:"desc"`
	Required   bool   `json:"required"`
	Positional bool   `json:"positional"`
}

// Result is the wire shape returned by /api/tools/:name. Mirrors
// tools.Result.
type Result struct {
	Text  string `json:"text,omitempty"`
	Table *Table `json:"table,omitempty"`
}

// Table is the structured part of a Result.
type Table struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
	Footer  []string   `json:"footer,omitempty"`
}

// NotFoundError signals "no such tool". InvokeTool implementations
// return it so the API layer can map to 404 without coupling to the
// registry.
type NotFoundError struct{ Name string }

func (e NotFoundError) Error() string { return "tool not found: " + e.Name }
