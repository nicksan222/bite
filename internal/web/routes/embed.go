package routes

import (
	"embed"
	"html/template"
	"io/fs"
	"path"
	"strings"
)

// assetsFS holds every static asset (CSS, JS, htmx.min.js) the dashboard
// serves and every Go template it renders. Both subtrees ship in the
// binary, so `bite web` is fully self-contained — no node_modules, no
// runtime CDN dependency.
//
//go:embed views/*.html static/*
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
	entries, err := fs.ReadDir(assetsFS, "views")
	if err != nil {
		panic("web/routes: read views/: " + err.Error())
	}
	out := make(map[string]*template.Template, len(entries))
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || name == "layout.html" || !strings.HasSuffix(name, ".html") {
			continue
		}
		t, err := template.ParseFS(assetsFS, "views/layout.html", path.Join("views", name))
		if err != nil {
			panic("web/routes: parse " + name + ": " + err.Error())
		}
		out[name] = t
	}
	return out
}

func mustSub(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		panic("web/routes: embed sub " + dir + ": " + err.Error())
	}
	return sub
}
