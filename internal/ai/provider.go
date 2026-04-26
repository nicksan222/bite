package ai

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/cloudwego/eino/components/model"
)

// ProviderSpec describes one AI backend. Each provider file registers
// its spec from init() so the rest of the codebase never switches on
// provider name.
type ProviderSpec struct {
	Name           Provider
	DefaultModel   string
	CredentialEnv  string // empty = no credential required
	BaseURLEnv     string // empty = no endpoint override supported
	DefaultBaseURL string
	Priority       int // lower wins during auto-detect

	// Build may assume Validate has passed against the same ProviderConfig.
	Build func(ctx context.Context, cfg ProviderConfig) (model.ToolCallingChatModel, error)
}

// ProviderConfig is the resolved input passed to ProviderSpec.Build.
type ProviderConfig struct {
	APIKey    string
	BaseURL   string
	Model     string
	MaxTokens int
}

var ErrNoProviderConfigured = errors.New("no AI provider configured")

// MissingCredentialError lets consumers errors.As against a typed value
// without knowing the provider implementation.
type MissingCredentialError struct {
	Provider Provider
	EnvVar   string
}

func (e *MissingCredentialError) Error() string {
	return fmt.Sprintf("missing %s — export your key, or add it to a .env file:\n\n  export %s=…", e.EnvVar, e.EnvVar)
}

func (s ProviderSpec) Validate(c ProviderConfig) error {
	if c.Model == "" {
		return fmt.Errorf("%s: missing model id", s.Name)
	}
	if s.CredentialEnv != "" && c.APIKey == "" {
		return &MissingCredentialError{Provider: s.Name, EnvVar: s.CredentialEnv}
	}
	return nil
}

// LoadConfig reads the spec's env vars. modelOverride wins over
// DefaultModel; pass "" to take the default.
func (s ProviderSpec) LoadConfig(modelOverride string, maxTokens int) ProviderConfig {
	cfg := ProviderConfig{Model: modelOverride, MaxTokens: maxTokens}
	if cfg.Model == "" {
		cfg.Model = s.DefaultModel
	}
	if s.CredentialEnv != "" {
		cfg.APIKey = os.Getenv(s.CredentialEnv)
	}
	if s.BaseURLEnv != "" {
		cfg.BaseURL = os.Getenv(s.BaseURLEnv)
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = s.DefaultBaseURL
	}
	return cfg
}

// IsConfigured returns true vacuously for credential-free providers;
// those are opt-in via explicit BITE_PROVIDER selection in Resolve.
func (s ProviderSpec) IsConfigured() bool {
	if s.CredentialEnv == "" {
		return true
	}
	return os.Getenv(s.CredentialEnv) != ""
}

var (
	providersMu sync.RWMutex
	providers   = map[Provider]ProviderSpec{}
)

// RegisterProvider panics on duplicate, missing Name, missing Build, or
// missing DefaultModel — every such case is a programming error.
func RegisterProvider(s ProviderSpec) {
	if s.Name == "" {
		panic("ai: ProviderSpec.Name is required")
	}
	if s.Build == nil {
		panic(fmt.Sprintf("ai: ProviderSpec.Build is required (provider=%s)", s.Name))
	}
	if s.DefaultModel == "" {
		panic(fmt.Sprintf("ai: ProviderSpec.DefaultModel is required (provider=%s)", s.Name))
	}
	providersMu.Lock()
	defer providersMu.Unlock()
	if _, exists := providers[s.Name]; exists {
		panic(fmt.Sprintf("ai: provider %q already registered", s.Name))
	}
	providers[s.Name] = s
}

func Lookup(name Provider) (ProviderSpec, bool) {
	providersMu.RLock()
	defer providersMu.RUnlock()
	s, ok := providers[name]
	return s, ok
}

// Providers returns every registered spec, sorted by Priority then Name.
func Providers() []ProviderSpec {
	providersMu.RLock()
	defer providersMu.RUnlock()
	out := make([]ProviderSpec, 0, len(providers))
	for _, s := range providers {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Resolve picks a provider for an optional explicit choice:
//
//  1. explicit, if registered.
//  2. First credentialed provider whose env key is set.
//  3. First registered provider — so missing-key errors point at the
//     obvious default.
func Resolve(explicit Provider) (ProviderSpec, error) {
	if explicit != "" {
		s, ok := Lookup(explicit)
		if !ok {
			return ProviderSpec{}, fmt.Errorf("ai: unknown provider %q", explicit)
		}
		return s, nil
	}
	all := Providers()
	for _, s := range all {
		if s.CredentialEnv != "" && s.IsConfigured() {
			return s, nil
		}
	}
	if len(all) == 0 {
		return ProviderSpec{}, errors.New("ai: no providers registered")
	}
	return all[0], nil
}

// AnyConfigured returns nil when at least one credentialed provider has
// its env key set; otherwise wraps ErrNoProviderConfigured with the env
// vars to try. Credential-free providers are excluded — they're opt-in.
func AnyConfigured() error {
	var missing []string
	for _, s := range Providers() {
		if s.CredentialEnv == "" {
			continue
		}
		if s.IsConfigured() {
			return nil
		}
		missing = append(missing, s.CredentialEnv)
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("%w — set one of: %s", ErrNoProviderConfigured, strings.Join(missing, ", "))
}
