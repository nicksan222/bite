package tools

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

// ─── Confirm ─────────────────────────────────────────────────────────────────

func TestConfirm_yesAccepts(t *testing.T) {
	for _, in := range []string{"y\n", "Y\n", "yes\n", "YES\n", "  y  \n"} {
		var out bytes.Buffer
		assert.True(t, Confirm(strings.NewReader(in), &out, "ok?"), "input %q should accept", in)
		assert.Contains(t, out.String(), "ok? [y/N]")
	}
}

func TestConfirm_noAndEmptyReject(t *testing.T) {
	for _, in := range []string{"n\n", "N\n", "no\n", "\n", "maybe\n"} {
		var out bytes.Buffer
		assert.False(t, Confirm(strings.NewReader(in), &out, "ok?"), "input %q should reject", in)
	}
}

func TestConfirm_eofReturnsFalse(t *testing.T) {
	// EOF (non-interactive stdin, piped script) must not greenlight a
	// destructive install — default no is the only safe choice.
	var out bytes.Buffer
	assert.False(t, Confirm(strings.NewReader(""), &out, "ok?"))
}

// ─── consent in the SQLite preferences table ────────────────────────────────

func newConsentStore(t *testing.T) db.Storer {
	t.Helper()
	store, err := db.Open(t.Context(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOllamaConsent_recordAndRead(t *testing.T) {
	ctx := t.Context()
	store := newConsentStore(t)
	require.False(t, HasOllamaConsent(ctx, store), "fresh DB → no consent")

	require.NoError(t, RecordOllamaConsent(ctx, store))
	assert.True(t, HasOllamaConsent(ctx, store), "after Record, HasConsent must report true")
}

func TestOllamaConsent_recordIsIdempotent(t *testing.T) {
	ctx := t.Context()
	store := newConsentStore(t)
	require.NoError(t, RecordOllamaConsent(ctx, store))
	require.NoError(t, RecordOllamaConsent(ctx, store), "second Record over an existing row must upsert, not fail")
	assert.True(t, HasOllamaConsent(ctx, store))
}

// TestOllamaConsent_consentSurvivesNewStoreOpen pins the portability
// guarantee: a fresh Store on the same DSN must see the prior consent.
// If this regresses, copying the .db file no longer carries the flag.
func TestOllamaConsent_consentSurvivesNewStoreOpen(t *testing.T) {
	dsn := t.TempDir() + "/bite.db"

	store1, err := db.Open(t.Context(), dsn)
	require.NoError(t, err)
	require.NoError(t, RecordOllamaConsent(t.Context(), store1))
	require.NoError(t, store1.Close())

	store2, err := db.Open(t.Context(), dsn)
	require.NoError(t, err)
	defer store2.Close()
	assert.True(t, HasOllamaConsent(t.Context(), store2))
}

// TestHasOllamaConsent_storeErrorReturnsFalse pins the safety
// behaviour: any DB error must be treated as "not consented" so we
// re-prompt rather than silently bootstrapping based on a half-known
// state.
func TestHasOllamaConsent_storeErrorReturnsFalse(t *testing.T) {
	assert.False(t, HasOllamaConsent(context.Background(), errStore{}))
}

type errStore struct{ db.Storer }

func (errStore) GetPreference(context.Context, string) (string, bool, error) {
	return "", false, errors.New("boom")
}
