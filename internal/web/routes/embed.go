package routes

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"path"
	"strings"
)

// assetsFS holds every static asset (CSS, JS, htmx.min.js) the dashboard
// serves and every Go template it renders. The bare `static` directive
// is recursive, so adding e.g. static/img/ later does not silently drop
// files. The views pattern is restricted to *.html so editor backups or
// .DS_Store leaks never end up in the binary.
//
//go:embed views/*.html static
var assetsFS embed.FS

// pageTemplates maps a page filename ("chat.html") to the layout pre-bound
// with that page's "content" block. Parsing happens once at init —
// template errors are programmer mistakes, so they fail startup loudly
// rather than the first request that hits them.
var pageTemplates = mustParsePages()

// staticFS is the static/ subtree, resolved once. fs.Sub on a literal
// embed prefix can only fail if the prefix is malformed, so we panic on
// error rather than ship a fallback handler nobody will ever exercise.
var staticFS = mustSub(assetsFS, "static")

func mustParsePages() map[string]*template.Template {
	out, err := parsePages(assetsFS)
	if err != nil {
		panic("web/routes: " + err.Error())
	}
	return out
}

// parsePages walks views/ and returns one layout-bound template per page.
// Split from mustParsePages so a unit test can drive it with an in-memory
// FS and assert the contract (skips layout.html, requires "content" block).
func parsePages(src fs.FS) (map[string]*template.Template, error) {
	entries, err := fs.ReadDir(src, "views")
	if err != nil {
		return nil, fmt.Errorf("read views/: %w", err)
	}
	out := make(map[string]*template.Template, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "layout.html" || !strings.HasSuffix(name, ".html") {
			continue
		}
		t, err := template.ParseFS(src, "views/layout.html", path.Join("views", name))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		// layout.html invokes {{template "content" .}} — a page that omits
		// the {{define "content"}} block parses fine but blows up at the
		// first request. Catch it here so the failure is loud at boot.
		if t.Lookup("content") == nil {
			return nil, fmt.Errorf("%s is missing a {{define \"content\"}} block", name)
		}
		out[name] = t
	}
	return out, nil
}

func mustSub(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		panic("web/routes: embed sub " + dir + ": " + err.Error())
	}
	return sub
}
