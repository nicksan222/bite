package routes

import (
	"embed"
	"html/template"
	"io/fs"
)

// assetsFS holds every static asset (CSS, JS, htmx.min.js) the dashboard
// serves and every Go template it renders. Both subtrees ship in the
// binary, so `bite web` is fully self-contained — no node_modules, no
// runtime CDN dependency.
//
//go:embed views/*.html static/*
var assetsFS embed.FS

// baseTemplates is the parsed layout, parsed once at init. Templates
// live in views/ and ship in the binary, so a parse error is a programmer
// mistake caught at startup — never at request time.
var baseTemplates = template.Must(template.ParseFS(assetsFS, "views/layout.html"))

// staticFS is the static/ subtree, resolved once. fs.Sub on a literal
// embed prefix can only fail if the prefix is malformed, so we panic on
// error rather than ship a fallback handler nobody will ever exercise.
var staticFS = mustSub(assetsFS, "static")

func mustSub(parent fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(parent, dir)
	if err != nil {
		panic("web/routes: embed sub " + dir + ": " + err.Error())
	}
	return sub
}
