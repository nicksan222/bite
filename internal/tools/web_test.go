package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/nicksan222/bite/internal/web"
)

// The web tool is registered at init; these tests pin its surface so a
// rename or accidental SkipAI/SkipSlash flip is caught immediately.
func TestWebTool_registration(t *testing.T) {
	w, ok := Get("web")
	require.True(t, ok, "web tool must be registered")
	require.True(t, w.SkipAI, "web is a launcher; the model must not call it")
	require.True(t, w.SkipSlash, "web is a launcher; /web inside chat would be wrong")

	host, foundHost := paramByName(w.Params, "host")
	require.True(t, foundHost)
	require.Equal(t, ParamString, host.Type)
	require.Equal(t, "127.0.0.1", host.Default, "default must keep the server local")

	port, foundPort := paramByName(w.Params, "port")
	require.True(t, foundPort)
	require.Equal(t, ParamInt, port.Type)
	require.Equal(t, int64(8787), port.Default)
}

// webToolList must drop SkipAI tools — the HTTP surface is a model-style
// surface, so launchers (chat, web itself) would be recursive footguns.
func TestWebToolList_excludesSkipAI(t *testing.T) {
	got := webToolList()
	for _, m := range got {
		require.NotEqual(t, "chat", m.Name, "chat is SkipAI; must not appear over HTTP")
		require.NotEqual(t, "web", m.Name, "web is SkipAI; must not appear over HTTP")
	}
	// Sanity: a known non-SkipAI tool is present.
	require.True(t, hasTool(got, "meals_today"))
}

// invokeRegisteredTool must return web.NotFoundError for unknown names,
// so the API layer can map to 404 cleanly.
func TestInvokeRegisteredTool_unknown(t *testing.T) {
	_, err := invokeRegisteredTool(context.Background(), Deps{}, "definitely_not_a_tool", nil)
	var nf web.NotFoundError
	require.ErrorAs(t, err, &nf)
}

// invokeRegisteredTool must refuse SkipAI tools too — same blast radius
// as the listing exclusion.
func TestInvokeRegisteredTool_skipAIBlocked(t *testing.T) {
	_, err := invokeRegisteredTool(context.Background(), Deps{}, "chat", nil)
	var nf web.NotFoundError
	require.ErrorAs(t, err, &nf, "SkipAI tools must look like 404s, not 500s")
}

// toWebResult: a Result with Text+Table flows through unchanged. Catches
// any future regression where one half is dropped (e.g. someone forgets
// to copy Footer).
func TestToWebResult_roundtrip(t *testing.T) {
	in := Result{
		Text: "hello",
		Table: &Table{
			Headers: []string{"a", "b"},
			Rows:    [][]string{{"1", "2"}},
			Footer:  []string{"sum", "3"},
		},
	}
	got := toWebResult(in)
	require.Equal(t, "hello", got.Text)
	require.NotNil(t, got.Table)
	require.Equal(t, in.Table.Headers, got.Table.Headers)
	require.Equal(t, in.Table.Rows, got.Table.Rows)
	require.Equal(t, in.Table.Footer, got.Table.Footer)
}

// HTMX form posts arrive with string-shaped numbers. invokeRegisteredTool
// must coerce them per the param's declared type before the tool runs —
// otherwise log_meal's kcal=300 silently drops to 0. Regression test.
func TestInvokeRegisteredTool_coercesFormStrings(t *testing.T) {
	withCleanRegistry(t, func() {
		var got Args
		Register(Tool{
			Name:        "echo_args",
			Summary:     "Capture coerced args.",
			Description: "Test fixture for coerceWebArgs.",
			Examples:    []Example{{Cmd: "bite echo_args", Desc: "fixture only"}},
			Params: []Param{
				{Name: "kcal", Type: ParamFloat, Desc: "Float-typed param."},
				{Name: "count", Type: ParamInt, Desc: "Int-typed param."},
				{Name: "on", Type: ParamBool, Desc: "Bool-typed param."},
			},
			Run: func(_ context.Context, _ Deps, a Args) (Result, error) {
				got = a
				return Result{Text: "ok"}, nil
			},
		})
		_, err := invokeRegisteredTool(context.Background(), Deps{}, "echo_args", map[string]any{
			"kcal":  "300",
			"count": "5",
			"on":    "true",
		})
		require.NoError(t, err)
		require.InEpsilon(t, 300.0, got.Float("kcal"), 0.0001)
		require.Equal(t, int64(5), got.Int("count"))
		require.True(t, got.Bool("on"))
	})
}

// paramTypeName covers every enum value so future SPA / JSON clients can
// rely on the wire string staying stable.
func TestParamTypeName(t *testing.T) {
	cases := []struct {
		in   ParamType
		want string
	}{
		{ParamString, "string"},
		{ParamInt, "int"},
		{ParamFloat, "float"},
		{ParamBool, "bool"},
		{ParamStringList, "string_list"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, paramTypeName(c.in))
	}
}

func paramByName(ps []Param, name string) (Param, bool) {
	for _, p := range ps {
		if p.Name == name {
			return p, true
		}
	}
	return Param{}, false
}

func hasTool(ms []web.ToolMeta, name string) bool {
	for _, m := range ms {
		if m.Name == name {
			return true
		}
	}
	return false
}
