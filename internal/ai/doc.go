// Package ai is bite's AI layer.
//
// # Files
//
//   - client.go   — Client (streaming chat) and the Streamer interface.
//   - messages.go — Message, Attachment, and eino schema conversion.
//   - claude.go   — Claude model factory (bite is Claude-only by design).
//   - analyze.go  — AnalyzeMeal: any mix of text, images, audio, and video.
//
// bite is Claude-only by design. If you ever need a second provider, add a
// sibling file (e.g. openai.go) — don't build a generic provider system
// speculatively.
//
// The Streamer interface allows callers to inject mocks for testing without
// a real Anthropic network call.
package ai
