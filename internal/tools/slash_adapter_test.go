package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDispatch_unknownCommand(t *testing.T) {
	withCleanRegistry(t, func() {
		out := Dispatch(context.Background(), Deps{}, "/nope foo")
		require.Error(t, out.ParseError)
		assert.Contains(t, out.ParseError.Error(), "unknown command")
	})
}

func TestDispatch_help(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("alpha"))
		Register(noopTool("beta"))
		out := Dispatch(context.Background(), Deps{}, "/help")
		require.NoError(t, out.ParseError)
		assert.Contains(t, out.Result.Text, "/alpha")
		assert.Contains(t, out.Result.Text, "/beta")
	})
}

func TestDispatch_positional(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name: "say", Summary: "s", Description: "d",
			Params: []Param{{Name: "msg", Type: ParamString, Required: true, Positional: true}},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})
		out := Dispatch(context.Background(), Deps{}, "/say hello")
		require.NoError(t, out.ParseError)
		require.NoError(t, out.RunError)
		assert.Equal(t, "hello", seen.String("msg"))
	})
}

func TestDispatch_quotedPositional(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name: "say", Summary: "s", Description: "d",
			Params: []Param{{Name: "msg", Type: ParamString, Required: true, Positional: true}},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})
		out := Dispatch(context.Background(), Deps{}, `/say "hello world"`)
		require.NoError(t, out.ParseError)
		assert.Equal(t, "hello world", seen.String("msg"))
	})
}

func TestDispatch_keyedArgs(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name: "set", Summary: "s", Description: "d",
			Params: []Param{
				{Name: "kcal", Type: ParamFloat},
				{Name: "protein_g", Type: ParamFloat},
			},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})
		// Mix snake_case and kebab-case keys.
		out := Dispatch(context.Background(), Deps{}, "/set kcal=2000 protein-g=150")
		require.NoError(t, out.ParseError)
		assert.Equal(t, 2000.0, seen.Float("kcal"))
		assert.Equal(t, 150.0, seen.Float("protein_g"))
	})
}

func TestDispatch_missingRequired(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name: "say", Summary: "s", Description: "d",
			Params: []Param{{Name: "msg", Type: ParamString, Required: true}},
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
				return Result{Text: "ok"}, nil
			},
		})
		out := Dispatch(context.Background(), Deps{}, "/say")
		require.Error(t, out.ParseError)
		assert.Contains(t, out.ParseError.Error(), "missing required")
	})
}

func TestDispatch_positionalAfterKeyed_errors(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name: "x", Summary: "s", Description: "d",
			Params: []Param{
				{Name: "a", Type: ParamString, Positional: true},
				{Name: "b", Type: ParamString},
			},
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) { return Result{Text: "ok"}, nil },
		})
		out := Dispatch(context.Background(), Deps{}, "/x b=foo bar")
		require.Error(t, out.ParseError)
	})
}

func TestDispatch_unknownKey(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("a"))
		out := Dispatch(context.Background(), Deps{}, "/a foo=1")
		require.Error(t, out.ParseError)
		assert.Contains(t, out.ParseError.Error(), "unknown parameter")
	})
}

func TestNewSlashHandler_returnsRenderedTextOnSuccess(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name: "show", Summary: "s", Description: "d",
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
				return Result{Text: "hello"}, nil
			},
		})
		h := NewSlashHandler(Deps{})
		out, err := h(context.Background(), "/show")
		require.NoError(t, err)
		assert.Equal(t, "hello", out)
	})
}

func TestNewSlashHandler_surfacesParseError(t *testing.T) {
	withCleanRegistry(t, func() {
		h := NewSlashHandler(Deps{})
		_, err := h(context.Background(), "/nope")
		require.Error(t, err)
	})
}

func TestNewSlashHandler_surfacesRunError(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name: "boom", Summary: "s", Description: "d",
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
				return Result{}, assert.AnError
			},
		})
		h := NewSlashHandler(Deps{})
		_, err := h(context.Background(), "/boom")
		require.Error(t, err)
	})
}

func TestDispatch_runError(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(Tool{
			Name: "boom", Summary: "s", Description: "d",
			Run: func(_ context.Context, _ Deps, _ Args) (Result, error) {
				return Result{}, assert.AnError
			},
		})
		out := Dispatch(context.Background(), Deps{}, "/boom")
		require.NoError(t, out.ParseError)
		require.Error(t, out.RunError)
	})
}

func TestTokeniseSlash_quotedAndPlain(t *testing.T) {
	got, err := tokeniseSlash(`a "b c" d=1`)
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b c", "d=1"}, got)
}

func TestTokeniseSlash_unterminatedQuote(t *testing.T) {
	_, err := tokeniseSlash(`a "b`)
	assert.Error(t, err)
}

func TestDispatch_unterminatedQuoteSurfacesParseError(t *testing.T) {
	withCleanRegistry(t, func() {
		Register(noopTool("alpha"))
		out := Dispatch(context.Background(), Deps{}, `/alpha "unterminated`)
		require.Error(t, out.ParseError)
	})
}

func TestDispatch_repeatedKeyAppendsToList(t *testing.T) {
	withCleanRegistry(t, func() {
		var seen Args
		Register(Tool{
			Name: "many", Summary: "s", Description: "d",
			Params: []Param{{Name: "tag", Type: ParamStringList}},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				seen = a
				return Result{Text: "ok"}, nil
			},
		})
		out := Dispatch(context.Background(), Deps{}, "/many tag=red tag=blue")
		require.NoError(t, out.ParseError)
		assert.Equal(t, []string{"red", "blue"}, seen.StringList("tag"))
	})
}

func TestIsKeyValue(t *testing.T) {
	assert.True(t, isKeyValue("kcal=2000"))
	assert.True(t, isKeyValue("protein-g=150"))
	assert.True(t, isKeyValue("snake_case=1"))
	assert.False(t, isKeyValue("noequals"))
	assert.False(t, isKeyValue("=novalue"))
	assert.False(t, isKeyValue("=1=2"))
}
