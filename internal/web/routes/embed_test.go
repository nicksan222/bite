package routes

import (
	"maps"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/require"
)

// TestParsePages_loadsAllPages exercises the happy path with the same
// shape the embedded FS uses: a layout.html plus N page templates.
func TestParsePages_loadsAllPages(t *testing.T) {
	src := fstest.MapFS{
		"views/layout.html": &fstest.MapFile{Data: []byte(`{{define "layout"}}<html><body>{{template "content" .}}</body></html>{{end}}`)},
		"views/foo.html":    &fstest.MapFile{Data: []byte(`{{define "content"}}foo{{end}}`)},
		"views/bar.html":    &fstest.MapFile{Data: []byte(`{{define "content"}}bar{{end}}`)},
	}
	got, err := parsePages(src)
	require.NoError(t, err)
	require.ElementsMatch(t, []string{"foo.html", "bar.html"}, slices.Collect(maps.Keys(got)))
}

// TestParsePages_skipsLayout proves layout.html is never returned as a
// page — it's the wrapper, not a destination.
func TestParsePages_skipsLayout(t *testing.T) {
	src := fstest.MapFS{
		"views/layout.html": &fstest.MapFile{Data: []byte(`{{define "layout"}}{{template "content" .}}{{end}}`)},
		"views/page.html":   &fstest.MapFile{Data: []byte(`{{define "content"}}p{{end}}`)},
	}
	got, err := parsePages(src)
	require.NoError(t, err)
	require.NotContains(t, got, "layout.html")
}

// TestParsePages_missingContentBlock locks in the boot-time guard: a
// page that forgets {{define "content"}} parses fine but is unusable —
// fail loudly with the offending filename so the developer sees it
// before the first request.
func TestParsePages_missingContentBlock(t *testing.T) {
	src := fstest.MapFS{
		"views/layout.html": &fstest.MapFile{Data: []byte(`{{define "layout"}}{{template "content" .}}{{end}}`)},
		"views/broken.html": &fstest.MapFile{Data: []byte(`<p>oops</p>`)},
	}
	_, err := parsePages(src)
	require.Error(t, err)
	require.Contains(t, err.Error(), "broken.html")
}
