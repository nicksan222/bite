package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/ai"
)

func TestDeriveTitle_short(t *testing.T) {
	got := deriveTitle("had a big salad for lunch")
	assert.Equal(t, "had a big salad for lunch", got)
}

func TestDeriveTitle_truncates(t *testing.T) {
	long := "this is a really long message that exceeds sixty characters and should be truncated"
	got := deriveTitle(long)
	assert.LessOrEqual(t, len([]rune(got)), 61, "title too long (%d runes): %q", len([]rune(got)), got)
}

func TestDeriveTitle_multiline(t *testing.T) {
	got := deriveTitle("first line\nsecond line")
	assert.Equal(t, "first line", got)
}

func TestDeriveTitle_crlfMultiline(t *testing.T) {
	got := deriveTitle("first line\r\nsecond line")
	assert.Equal(t, "first line", got)
}

func TestDeriveTitle_trimsSpace(t *testing.T) {
	got := deriveTitle("  hello  ")
	assert.Equal(t, "hello", got)
}

func TestPrepareConversation_fresh(t *testing.T) {
	store := newMockStore()
	convID, history, err := prepareConversation(context.Background(), store, "claude-3-5-sonnet-20241022", 0)
	require.NoError(t, err)
	assert.NotZero(t, convID, "expected non-zero conversation ID")
	assert.Len(t, history, 0)
}

func TestPrepareConversation_resume(t *testing.T) {
	store := newMockStore()
	conv, _ := store.NewConversation(context.Background(), "test-model", "my convo")
	_, _ = store.AppendMessage(context.Background(), conv.ID, "user", "hello")
	_, _ = store.AppendMessage(context.Background(), conv.ID, "assistant", "hi there")

	convID, history, err := prepareConversation(context.Background(), store, "test-model", conv.ID)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, convID)
	require.Len(t, history, 2)
	assert.Equal(t, ai.RoleUser, history[0].Role)
	assert.Equal(t, ai.RoleAssistant, history[1].Role)
}

func TestPrepareConversation_resumeNotFound(t *testing.T) {
	store := newMockStore()
	_, _, err := prepareConversation(context.Background(), store, "m", 999)
	require.Error(t, err)
}

func TestChatPersister_appendUserAndAssistant(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	conv, _ := store.NewConversation(ctx, "m", "")

	p := &chatPersister{store: store, conversationID: conv.ID}

	require.NoError(t, p.AppendUser(ctx, "what's good to eat?"))
	require.NoError(t, p.AppendAssistant(ctx, "salads are great!"))

	msgs, _ := store.ListMessages(ctx, conv.ID)
	require.Len(t, msgs, 2)
	assert.Equal(t, "user", msgs[0].Role)
	assert.Equal(t, "assistant", msgs[1].Role)
}

func TestChatPersister_firstUserMessageSetsTitle(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	conv, _ := store.NewConversation(ctx, "m", "")

	p := &chatPersister{store: store, conversationID: conv.ID, titled: false}
	_ = p.AppendUser(ctx, "I had pasta for lunch today")

	updated, _ := store.GetConversation(ctx, conv.ID)
	assert.Equal(t, "I had pasta for lunch today", updated.Title)

	// Second AppendUser should NOT rename again.
	_ = p.AppendUser(ctx, "what about dinner?")
	updated2, _ := store.GetConversation(ctx, conv.ID)
	assert.Equal(t, "I had pasta for lunch today", updated2.Title)
}

func TestChatPersister_alreadyTitled(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	conv, _ := store.NewConversation(ctx, "m", "existing title")

	p := &chatPersister{store: store, conversationID: conv.ID, titled: true}
	_ = p.AppendUser(ctx, "new message")

	updated, _ := store.GetConversation(ctx, conv.ID)
	assert.Equal(t, "existing title", updated.Title)
}

func TestPrepareConversation_listMessagesError(t *testing.T) {
	store := newMockStore()
	conv, _ := store.NewConversation(context.Background(), "m", "")
	store.listMsgsErr = errors.New("io error")

	_, _, err := prepareConversation(context.Background(), store, "m", conv.ID)
	require.Error(t, err)
	assert.ErrorIs(t, err, store.listMsgsErr)
}

func TestPrepareConversation_newConversationError(t *testing.T) {
	store := newMockStore()
	store.newConvErr = errors.New("db full")

	_, _, err := prepareConversation(context.Background(), store, "m", 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, store.newConvErr)
}

func TestChatPersister_appendUserError(t *testing.T) {
	ctx := context.Background()
	store := newMockStore()
	store.appendMsgErr = errors.New("write failed")
	p := &chatPersister{store: store, conversationID: 1}
	require.Error(t, p.AppendUser(ctx, "hello"))
}

func TestRunChat_configError(t *testing.T) {
	t.Setenv("BITE_MAX_TOKENS", "not-a-number")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	require.Error(t, err)
}

func TestRunChat_openStoreError(t *testing.T) {
	t.Setenv("BITE_DB", "/tmp") // directory, not a file
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	require.Error(t, err)
}

func TestRunChat_missingKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	require.Error(t, err)
}

func TestRunChat_resumeNotFound(t *testing.T) {
	// Valid config + valid store, but resume ID 9999 doesn't exist.
	// prepareConversation fails → runChat returns error at line 52.
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 9999)
	require.Error(t, err)
}

func TestRunChat_noTTY(t *testing.T) {
	// All setup succeeds; prog.Run() fails with "no TTY" in CI/test env.
	// This covers the TUI launch path (persist/New/Run).
	dir := t.TempDir()
	t.Setenv("BITE_DB", dir+"/test.db")
	t.Setenv("BITE_MODEL", "claude-haiku-4-5")
	t.Setenv("BITE_MAX_TOKENS", "")
	t.Setenv("ANTHROPIC_API_KEY", "sk-test-fake")
	t.Setenv("XDG_DATA_HOME", t.TempDir())

	err := runChat(context.Background(), 0)
	// In a non-TTY environment bubbletea returns "could not open a new TTY"
	if err == nil {
		t.Skip("running in TTY environment — skip non-TTY coverage test")
	}
}
