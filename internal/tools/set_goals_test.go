package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/db"
)

func TestSetGoals_setOne(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)

	res, err := MustGet("set_goals").Run(ctx, deps, NewArgs(map[string]any{
		"kcal": 2000.0,
	}))
	require.NoError(t, err)
	assert.Contains(t, res.Text, "2000")

	g, err := deps.Store.GetGoals(ctx)
	require.NoError(t, err)
	require.NotNil(t, g.Kcal)
	assert.Equal(t, 2000.0, *g.Kcal)
	assert.Nil(t, g.ProteinG)
}

func TestSetGoals_omitPreservesExisting(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	existing := 1800.0
	_, err := deps.Store.SetGoals(ctx, db.GoalInput{Kcal: &existing})
	require.NoError(t, err)

	_, err = MustGet("set_goals").Run(ctx, deps, NewArgs(map[string]any{
		"protein_g": 150.0,
	}))
	require.NoError(t, err)

	g, err := deps.Store.GetGoals(ctx)
	require.NoError(t, err)
	require.NotNil(t, g.Kcal, "kcal should be preserved when omitted")
	assert.Equal(t, 1800.0, *g.Kcal)
	require.NotNil(t, g.ProteinG)
	assert.Equal(t, 150.0, *g.ProteinG)
}

func TestSetGoals_zeroClears(t *testing.T) {
	ctx := context.Background()
	deps := freshDeps(t)
	v := 1800.0
	_, err := deps.Store.SetGoals(ctx, db.GoalInput{Kcal: &v})
	require.NoError(t, err)

	_, err = MustGet("set_goals").Run(ctx, deps, NewArgs(map[string]any{
		"kcal": 0.0,
	}))
	require.NoError(t, err)

	g, err := deps.Store.GetGoals(ctx)
	require.NoError(t, err)
	assert.Nil(t, g.Kcal, "kcal=0 should clear")
}
