package tools

import (
	"context"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAITools_emptyRegistry(t *testing.T) {
	withCleanRegistry(t, func() {
		assert.Empty(t, AITools(Deps{}))
	})
}

func TestAITools_paramsTranslated(t *testing.T) {
	withCleanRegistry(t, func() {
		tt := noopTool("with_params")
		tt.Params = []Param{
			{Name: "title", Type: ParamString, Required: true},
			{Name: "kcal", Type: ParamFloat},
			{Name: "items", Type: ParamStringList},
			{Name: "count", Type: ParamInt},
			{Name: "active", Type: ParamBool},
		}
		Register(tt)

		got := AITools(Deps{})
		require.Len(t, got, 1)
		require.Equal(t, "with_params", got[0].Name)

		params := got[0].Parameters
		require.NotNil(t, params)
		assert.Equal(t, schema.String, params["title"].Type)
		assert.True(t, params["title"].Required)
		assert.Equal(t, schema.Number, params["kcal"].Type)
		assert.Equal(t, schema.Integer, params["count"].Type)
		assert.Equal(t, schema.Boolean, params["active"].Type)
		assert.Equal(t, schema.Array, params["items"].Type)
		require.NotNil(t, params["items"].ElemInfo)
		assert.Equal(t, schema.String, params["items"].ElemInfo.Type)
	})
}

func TestAITools_noParams_nilSchema(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("no_params"))
		got := AITools(Deps{})
		require.Len(t, got, 1)
		assert.Nil(t, got[0].Parameters)
	})
}

func TestAITools_ExecuteRoundTrip(t *testing.T) {
	withCleanRegistry(t, func() {
		var seenArgs Args
		tt := Tool{
			Name:        "echo",
			Summary:     "s",
			Description: "d",
			Params: []Param{
				{Name: "msg", Type: ParamString, Required: true},
				{Name: "n", Type: ParamInt},
			},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seenArgs = a
				return Result{Text: "hi " + a.String("msg")}, nil
			},
		}
		Register(tt)

		got := AITools(Deps{})
		require.Len(t, got, 1)

		out, err := got[0].Execute(context.Background(), `{"msg":"world","n":3}`)
		require.NoError(t, err)
		assert.Equal(t, "hi world", out)
		assert.Equal(t, "world", seenArgs.String("msg"))
		assert.Equal(t, int64(3), seenArgs.Int("n"))
	})
}

func TestAITools_ExecuteNoArgs(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("no_args"))
		got := AITools(Deps{})
		out, err := got[0].Execute(context.Background(), "")
		require.NoError(t, err)
		assert.Equal(t, "ok", out)
	})
}

func TestAITools_ExecuteInvalidJSON(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("bad_json"))
		got := AITools(Deps{})
		_, err := got[0].Execute(context.Background(), `{not json`)
		assert.Error(t, err)
	})
}

func TestRenderMarkdownTable(t *testing.T) {
	got := renderMarkdownTable(&Table{
		Headers: []string{"A", "B"},
		Rows:    [][]string{{"1", "2"}, {"3", "4"}},
		Footer:  []string{"sum", "10"},
	})
	assert.Contains(t, got, "| A | B |")
	assert.Contains(t, got, "| 1 | 2 |")
	assert.Contains(t, got, "| sum | 10 |")
}

func TestRenderForAI_textPlusTable(t *testing.T) {
	out := renderForAI(Result{
		Text: "hello",
		Table: &Table{
			Headers: []string{"X"},
			Rows:    [][]string{{"y"}},
		},
	})
	assert.Contains(t, out, "hello")
	assert.Contains(t, out, "| X |")
	assert.Contains(t, out, "| y |")
}
