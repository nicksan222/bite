package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestParseID_valid(t *testing.T) {
	id, err := parseID("42")
	require.NoError(t, err)
	assert.Equal(t, int64(42), id)
}

func TestParseID_invalid(t *testing.T) {
	_, err := parseID("abc")
	require.Error(t, err)
}

func TestListConversations_empty(t *testing.T) {
	store := newMockStore()
	var buf bytes.Buffer
	require.NoError(t, listConversations(context.Background(), &buf, store))
	assert.Contains(t, buf.String(), "ID")
}

func TestListConversations_shows_entries(t *testing.T) {
	store := newMockStore()
	_, _ = store.NewConversation(context.Background(), "claude-3", "breakfast talk")
	_, _ = store.NewConversation(context.Background(), "claude-3", "")

	var buf bytes.Buffer
	require.NoError(t, listConversations(context.Background(), &buf, store))
	out := buf.String()
	assert.Contains(t, out, "breakfast talk")
	assert.Contains(t, out, "(untitled)")
}

func TestShowConversation_printsMessages(t *testing.T) {
	store := newMockStore()
	conv, _ := store.NewConversation(context.Background(), "m", "test")
	_, _ = store.AppendMessage(context.Background(), conv.ID, "user", "what's for lunch?")
	_, _ = store.AppendMessage(context.Background(), conv.ID, "assistant", "salad!")

	var buf bytes.Buffer
	require.NoError(t, showConversation(context.Background(), &buf, store, conv.ID))
	out := buf.String()
	assert.Contains(t, out, "what's for lunch?")
	assert.Contains(t, out, "salad!")
}

func TestShowConversation_notFound(t *testing.T) {
	store := newMockStore()
	err := showConversation(context.Background(), &bytes.Buffer{}, store, 99)
	require.Error(t, err)
}

func TestDeleteConversation_removes(t *testing.T) {
	store := newMockStore()
	conv, _ := store.NewConversation(context.Background(), "m", "to delete")

	var buf bytes.Buffer
	require.NoError(t, deleteConversation(context.Background(), &buf, store, conv.ID))
	assert.Contains(t, buf.String(), "deleted")

	_, err := store.GetConversation(context.Background(), conv.ID)
	require.Error(t, err, "conversation should have been deleted")
}

func TestRenameConversation_updatesTitle(t *testing.T) {
	store := newMockStore()
	conv, _ := store.NewConversation(context.Background(), "m", "old title")

	var buf bytes.Buffer
	require.NoError(t, renameConversation(context.Background(), &buf, store, conv.ID, "new title"))

	updated, _ := store.GetConversation(context.Background(), conv.ID)
	assert.Equal(t, "new title", updated.Title)
}

func TestListConversations_storeError(t *testing.T) {
	store := newMockStore()
	store.listConvsErr = errors.New("db gone")
	err := listConversations(context.Background(), &bytes.Buffer{}, store)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db gone")
}

func TestShowConversation_listMessagesError(t *testing.T) {
	store := newMockStore()
	conv, _ := store.NewConversation(context.Background(), "m", "test")
	store.listMsgsErr = errors.New("disk full")

	err := showConversation(context.Background(), &bytes.Buffer{}, store, conv.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "disk full")
}

func TestDeleteConversation_notFound(t *testing.T) {
	store := newMockStore()
	err := deleteConversation(context.Background(), &bytes.Buffer{}, store, 9999)
	require.Error(t, err)
}

func TestWithStore_success(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	var called bool
	err := withStore(context.Background(), func(s db.Storer) error {
		called = true
		return nil
	})
	require.NoError(t, err)
	assert.True(t, called, "callback was not called")
}

func TestWithStore_openStoreError(t *testing.T) {
	// Valid config but DSN pointing to a directory → SQLite fails to open.
	// /tmp is a directory; SQLite can't use it as a database file.
	t.Setenv("BITE_DB", "/tmp")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")

	err := withStore(context.Background(), func(_ db.Storer) error { return nil })
	require.Error(t, err)
}

func TestWithStore_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := withStore(context.Background(), func(_ db.Storer) error { return nil })
	require.Error(t, err)
}

func TestWithStore_fnError(t *testing.T) {
	t.Setenv("BITE_DB", ":memory:")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := withStore(context.Background(), func(_ db.Storer) error {
		return errors.New("fn failed")
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fn failed")
}

func withTestFileStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dsn := fmt.Sprintf("%s/test.db", dir)
	t.Setenv("BITE_DB", dsn)
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	return dsn
}

func newCmdWithContext() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	return cmd
}

func TestRunConversationsList_integration(t *testing.T) {
	withTestFileStore(t)

	cmd := newCmdWithContext()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runConversationsList(cmd, nil))
}

func TestRunConversationShow_integration(t *testing.T) {
	withTestFileStore(t)

	// Create a conversation in the shared file DB.
	var convID string
	err := withStore(context.Background(), func(s db.Storer) error {
		conv, err := s.NewConversation(context.Background(), "m", "test")
		if err != nil {
			return err
		}
		convID = fmt.Sprintf("%d", conv.ID)
		return nil
	})
	require.NoError(t, err)

	cmd := newCmdWithContext()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runConversationShow(cmd, []string{convID}))
}

func TestRunConversationDelete_integration(t *testing.T) {
	withTestFileStore(t)

	var convID string
	err := withStore(context.Background(), func(s db.Storer) error {
		conv, err := s.NewConversation(context.Background(), "m", "to delete")
		if err != nil {
			return err
		}
		convID = fmt.Sprintf("%d", conv.ID)
		return nil
	})
	require.NoError(t, err)

	cmd := newCmdWithContext()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runConversationDelete(cmd, []string{convID}))
}

func TestRunConversationRename_integration(t *testing.T) {
	withTestFileStore(t)

	var convID string
	err := withStore(context.Background(), func(s db.Storer) error {
		conv, err := s.NewConversation(context.Background(), "m", "old")
		if err != nil {
			return err
		}
		convID = fmt.Sprintf("%d", conv.ID)
		return nil
	})
	require.NoError(t, err)

	cmd := newCmdWithContext()
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	require.NoError(t, runConversationRename(cmd, []string{convID, "new title"}))
}

func TestRenameConversation_notFound(t *testing.T) {
	store := newMockStore()
	err := renameConversation(context.Background(), &bytes.Buffer{}, store, 9999, "new name")
	require.Error(t, err)
}

func TestRunConversationShow_badID(t *testing.T) {
	cmd := newCmdWithContext()
	err := runConversationShow(cmd, []string{"not-a-number"})
	require.Error(t, err)
}

func TestRunConversationDelete_badID(t *testing.T) {
	cmd := newCmdWithContext()
	err := runConversationDelete(cmd, []string{"not-a-number"})
	require.Error(t, err)
}

func TestRunConversationRename_badID(t *testing.T) {
	cmd := newCmdWithContext()
	err := runConversationRename(cmd, []string{"not-a-number", "title"})
	require.Error(t, err)
}
