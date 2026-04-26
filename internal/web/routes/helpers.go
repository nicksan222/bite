package routes

import (
	"bytes"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// jsonError is the shape every API and HTMX handler returns for
// predictable client errors. Centralised so the {"error": …} envelope
// stays consistent across surfaces.
func jsonError(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

// render executes the named content template inside layout.html and
// writes the result. We render to a buffer first so a template error
// surfaces as a 500 rather than a half-written response with a 200.
//
// Each page defines its own "content" block. Cloning the parsed layout
// per call keeps the blocks isolated — without the clone, the second
// page parsed would shadow the first's "content" definition globally.
func render(c fiber.Ctx, contentTmpl string, data any) error {
	view, err := baseTemplates.Clone()
	if err != nil {
		return fiber.NewError(http.StatusInternalServerError, "clone templates: "+err.Error())
	}
	if _, err := view.ParseFS(assetsFS, "views/"+contentTmpl); err != nil {
		return fiber.NewError(http.StatusInternalServerError, "parse "+contentTmpl+": "+err.Error())
	}
	var buf bytes.Buffer
	if err := view.ExecuteTemplate(&buf, "layout", data); err != nil {
		return fiber.NewError(http.StatusInternalServerError, "render: "+err.Error())
	}
	return c.Type("html").Send(buf.Bytes())
}

// mergeArgs combines query-string and form values into a single raw map
// for InvokeTool. Form values take precedence — the typical HTMX pattern
// is hx-post with form data, where any querystring is incidental. Empty
// strings are dropped so absent-vs-empty stays meaningful upstream
// (matches set_goals' Has-vs-zero distinction).
func mergeArgs(q map[string]string, form map[string]string) map[string]any {
	out := make(map[string]any, len(q)+len(form))
	for k, v := range q {
		if v != "" {
			out[k] = v
		}
	}
	for k, v := range form {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

// formArgs returns the URL-encoded form body as a map. Fiber returns
// non-nil even for empty bodies, so callers don't need a nil-check.
func formArgs(c fiber.Ctx) map[string]string {
	out := map[string]string{}
	c.Request().PostArgs().VisitAll(func(k, v []byte) {
		out[string(k)] = string(v)
	})
	return out
}

// pageData is the layout's expected shape. Pages embed it and add their
// own fields via composition.
type pageData struct {
	Title  string
	Active string
}
