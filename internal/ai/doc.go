// Package ai is bite's AI layer.
//
// Adding a provider is a one-file change: drop a sibling next to
// claude.go that calls RegisterProvider in init(). NewClient, doctor
// checks, and resolution all consume the registry — they never switch
// on provider name. Each provider's Validate returns a typed
// *MissingCredentialError so callers get user-actionable messages
// without knowing the implementation.
package ai
