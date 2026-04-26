package tools

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

// freshDeps spins up a fresh in-memory SQLite store, fixed clock, and UTC loc.
// The store is closed via t.Cleanup.
func freshDeps(t *testing.T) Deps {
	t.Helper()
	store, err := db.Open(context.Background(), ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	fixed := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	return Deps{
		Store: store,
		Now:   func() time.Time { return fixed },
		Loc:   time.UTC,
	}
}
