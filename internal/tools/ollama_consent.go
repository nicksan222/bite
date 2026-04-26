package tools

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nicksan222/bite/internal/db"
)

const ollamaConsentKey = "ollama.consent.granted"

// HasOllamaConsent treats DB errors as "not consented" so a half-known
// state can't silently green-light a multi-GB install.
func HasOllamaConsent(ctx context.Context, store db.Storer) bool {
	v, ok, err := store.GetPreference(ctx, ollamaConsentKey)
	if err != nil || !ok {
		return false
	}
	return v == "true"
}

func RecordOllamaConsent(ctx context.Context, store db.Storer) error {
	return store.SetPreference(ctx, ollamaConsentKey, "true")
}

// Confirm defaults to no — bare Enter, EOF, or any unrecognised reply
// returns false so piped input never accidentally consents.
func Confirm(in io.Reader, out io.Writer, prompt string) bool {
	fmt.Fprint(out, prompt+" [y/N] ")
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
