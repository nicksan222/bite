package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestGetGoals_unset(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	res, err := MustGet("get_goals").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "Daily targets")
	assert.Contains(t, res.Text, "not set")
}

func TestGetGoals_setValues(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	v := 2000.0
	_, err := deps.Store.SetGoals(ctx, db.GoalInput{Kcal: &v})
	require.NoError(t, err)

	res, err := MustGet("get_goals").Run(ctx, deps, NewArgs(nil))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "2000")
}
