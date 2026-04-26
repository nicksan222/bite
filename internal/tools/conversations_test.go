package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConversationsList_empty(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("conversations_list").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "no conversations")
}

func TestConversationsList_returnsTable(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := deps.Store.NewConversation(ctx, "claude-x", "first")
	require.NoError(t, err)
	_, err = deps.Store.NewConversation(ctx, "claude-x", "second")
	require.NoError(t, err)

	res, err := MustGet("conversations_list").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	require.NotNil(t, res.Table)
	assert.Len(t, res.Table.Rows, 2)
}

func TestConversationsList_untitledFallback(t *testing.T) {
	// A conversation with an empty title (fresh chat before the first user
	// message sets the title) must render as "(untitled)" rather than blank.
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := deps.Store.NewConversation(ctx, "claude-x", "")
	require.NoError(t, err)

	res, err := MustGet("conversations_list").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	require.NotNil(t, res.Table)
	require.Len(t, res.Table.Rows, 1)
	assert.Equal(t, "(untitled)", res.Table.Rows[0][3])
}

func TestConversationShow_renders(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "claude-x", "t")
	require.NoError(t, err)
	_, err = deps.Store.AppendMessage(ctx, conv.ID, "user", "hi")
	require.NoError(t, err)
	_, err = deps.Store.AppendMessage(ctx, conv.ID, "assistant", "hello")
	require.NoError(t, err)

	res, err := MustGet("conversation_show").Run(ctx, deps, NewArgs(map[string]any{
		"conversation_id": float64(conv.ID),
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "## user")
	assert.Contains(t, res.Text, "hi")
	assert.Contains(t, res.Text, "hello")
}

func TestConversationShow_missing(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	_, err := MustGet("conversation_show").Run(ctx, deps, NewArgs(map[string]any{
		"conversation_id": float64(9999),
	}))
	require.Error(t, err)
}

func TestRenameConversation_updates(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)

	res, err := MustGet("rename_conversation").Run(ctx, deps, NewArgs(map[string]any{
		"conversation_id": float64(conv.ID),
		"title":           "renamed",
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "renamed")

	got, err := deps.Store.GetConversation(ctx, conv.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Title)
}

func TestDeleteConversation_removes(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)

	res, err := MustGet("delete_conversation").Run(ctx, deps, NewArgs(map[string]any{
		"conversation_id": float64(conv.ID),
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "deleted")

	_, err = deps.Store.GetConversation(ctx, conv.ID)
	require.Error(t, err)
}

func TestRenameConversation_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)
	require.NoError(t, deps.Store.Close()) // closing makes Rename surface a DB error

	_, err = MustGet("rename_conversation").Run(ctx, deps, NewArgs(map[string]any{
		"conversation_id": float64(conv.ID),
		"title":           "x",
	}))
	require.Error(t, err)
}

func TestDeleteConversation_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	conv, err := deps.Store.NewConversation(ctx, "m", "")
	require.NoError(t, err)
	require.NoError(t, deps.Store.Close())

	_, err = MustGet("delete_conversation").Run(ctx, deps, NewArgs(map[string]any{
		"conversation_id": float64(conv.ID),
	}))
	require.Error(t, err)
}

func TestConversationsList_storeError(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	require.NoError(t, deps.Store.Close())

	_, err := MustGet("conversations_list").Run(ctx, deps, NewArgs(nil))
	require.Error(t, err)
}
