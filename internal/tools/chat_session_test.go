package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareSession_freshConversation(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	id, history, err := PrepareSession(ctx, deps.Store, "claude-x", 0)
	require.NoError(t, err)
	assert.NotZero(t, id)
	assert.Empty(t, history)

	conv, err := deps.Store.GetConversation(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "claude-x", conv.Model)
}

func TestPrepareSession_resume(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	conv, err := deps.Store.NewConversation(ctx, "claude-x", "old")
	require.NoError(t, err)
	_, err = deps.Store.AppendMessage(ctx, conv.ID, "user", "hi")
	require.NoError(t, err)

	id, history, err := PrepareSession(ctx, deps.Store, "claude-x", conv.ID)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, id)
	require.Len(t, history, 1)
	assert.Equal(t, "hi", history[0].Content)
}

func TestPrepareSession_resumeMissing(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, _, err := PrepareSession(ctx, deps.Store, "claude-x", 9999)
	require.Error(t, err)
}

func TestChatPersister_appendUser_setsTitle(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)

	p := NewChatPersister(deps.Store, conv.ID, false)
	require.NoError(t, p.AppendUser(ctx, "first message that becomes the title"))

	got, err := deps.Store.GetConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Contains(t, got.Title, "first message")
}

func TestChatPersister_appendUser_doesNotRetitle(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "preserve")
	require.NoError(t, err)

	p := NewChatPersister(deps.Store, conv.ID, true)
	require.NoError(t, p.AppendUser(ctx, "later message"))

	got, err := deps.Store.GetConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Equal(t, "preserve", got.Title)
}

func TestChatPersister_appendAssistant(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)

	p := NewChatPersister(deps.Store, conv.ID, true)
	require.NoError(t, p.AppendAssistant(ctx, "reply"))

	msgs, err := deps.Store.ListMessages(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1)
	assert.Equal(t, "assistant", msgs[0].Role)
	assert.Equal(t, "reply", msgs[0].Content)
}

func TestChatPersister_appendUser_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)
	require.NoError(t, deps.Store.Close()) // makes AppendMessage error

	p := NewChatPersister(deps.Store, conv.ID, false)
	require.Error(t, p.AppendUser(ctx, "hi"))
}

func TestChatPersister_appendAssistant_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)
	require.NoError(t, deps.Store.Close())

	p := NewChatPersister(deps.Store, conv.ID, true)
	require.Error(t, p.AppendAssistant(ctx, "hi"))
}

func TestDeriveTitle_truncatesAndStripsNewlines(t *testing.T) {
	got := DeriveTitle("first line\nsecond line")
	assert.Equal(t, "first line", got)

	long := DeriveTitle("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	assert.LessOrEqual(t, len(long), 64)
	assert.Contains(t, long, "…")
}
