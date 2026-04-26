package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/config"
	"github.com/nicksan222/bite/internal/db"
)

// init wires one soft per-provider row + a hard "any configured" gate.
// Per-provider is soft because most users run just one — N-1 hard
// failures would be noise.
func init() {
	for _, spec := range ai.Providers() {
		RegisterCheck(providerCheck(spec))
	}

	RegisterCheck(Check{
		Name:     "provider: any configured",
		Severity: SeverityHard,
		Desc:     "at least one AI provider has credentials set",
		Run:      anyConfiguredCheck,
	})
}

// anyConfiguredCheck passes on credentialed env keys OR persisted
// Ollama consent — otherwise doctor would hard-fail a working Ollama
// setup.
func anyConfiguredCheck(ctx context.Context) (string, error) {
	if err := ai.AnyConfigured(); err == nil {
		return providerSummary(), nil
	}
	consented, err := readOllamaConsent(ctx)
	if err != nil {
		return "", err
	}
	if consented {
		return "Ollama consented (run `bite chat` to use it)", nil
	}
	return "", ai.AnyConfigured()
}

func readOllamaConsent(ctx context.Context) (bool, error) {
	cfg, err := config.Load()
	if err != nil {
		return false, err
	}
	store, err := db.Open(ctx, cfg.DSN)
	if err != nil {
		return false, err
	}
	defer store.Close()
	return HasOllamaConsent(ctx, store), nil
}

func providerCheck(spec ai.ProviderSpec) Check {
	desc := fmt.Sprintf("%s credentials are set", spec.Name)
	if spec.CredentialEnv == "" {
		desc = fmt.Sprintf("%s endpoint is reachable via env", spec.Name)
	}
	return Check{
		Name:     "provider: " + string(spec.Name),
		Severity: SeveritySoft,
		Desc:     desc,
		Run: func(_ context.Context) (string, error) {
			pcfg := spec.LoadConfig("", 0)
			if err := spec.Validate(pcfg); err != nil {
				var miss *ai.MissingCredentialError
				if errors.As(err, &miss) {
					return "", fmt.Errorf("%s not configured (set %s)", spec.Name, miss.EnvVar)
				}
				return "", err
			}
			return providerDetail(spec), nil
		},
	}
}

func providerDetail(spec ai.ProviderSpec) string {
	if spec.CredentialEnv != "" {
		return fmt.Sprintf("%s set, default model=%s", spec.CredentialEnv, spec.DefaultModel)
	}
	if spec.BaseURLEnv != "" {
		return fmt.Sprintf("%s defaults to %s, default model=%s", spec.BaseURLEnv, spec.DefaultBaseURL, spec.DefaultModel)
	}
	return fmt.Sprintf("default model=%s", spec.DefaultModel)
}

func providerSummary() string {
	var configured []string
	for _, spec := range ai.Providers() {
		if spec.CredentialEnv != "" && spec.IsConfigured() {
			configured = append(configured, string(spec.Name))
		}
	}
	if len(configured) == 0 {
		return "no credentialed provider — Ollama only via explicit BITE_PROVIDER"
	}
	return "configured: " + strings.Join(configured, ", ")
}
