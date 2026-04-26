package routes

import (
	"bytes"
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// jsonError is the shape every API and HTMX handler returns for
// predictable client errors. Centralised so the {"error": …} envelope
// stays consistent across surfaces.
func jsonError(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

// toolErrorStatus maps an InvokeTool error to its HTTP status. A
// NotFoundError means the tool name was unknown (404); anything else
// is treated as a client-side problem with the call (400). Both
// invokeTool and htmxTool reach for this so their status mapping can't
// drift.
func toolErrorStatus(err error) int {
	var nf NotFoundError
	if errors.As(err, &nf) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

// render executes the layout pre-bound with the named page's "content"
// block and writes the result. Templates are parsed once at init (see
// embed.go) so a missing page name here is a programmer mistake, not a
// runtime template error. Rendering goes through a buffer first so an
// execution error surfaces as a 500 rather than a half-written 200.
func render(c fiber.Ctx, contentTmpl string, data any) error {
	view, ok := pageTemplates[contentTmpl]
	if !ok {
		return fiber.NewError(http.StatusInternalServerError, "unknown page template: "+contentTmpl)
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
	addNonEmpty(out, q)
	addNonEmpty(out, form)
	return out
}

// addNonEmpty copies src entries into dst, skipping empty-string values.
// Splitting this out keeps mergeArgs' precedence rule (form overwrites
// query) literal — read top-to-bottom — without duplicating the filter.
func addNonEmpty(dst map[string]any, src map[string]string) {
	for k, v := range src {
		if v != "" {
			dst[k] = v
		}
	}
}

// formArgs returns the URL-encoded form body as a map. Fiber returns
// non-nil even for empty bodies, so callers don't need a nil-check.
func formArgs(c fiber.Ctx) map[string]string {
	out := map[string]string{}
	for k, v := range c.Request().PostArgs().All() {
		out[string(k)] = string(v)
	}
	return out
}

// pageData is the layout's expected shape. Title fills the <title> tag
// (and any layout copy that refers to it); Active controls the
// highlighted nav link in layout.html and must match one of the keys
// the layout's {{if eq .Active "…"}} branches against.
//
// Pages with extra payload (cards, lists) embed pageData and add their
// own fields via composition.
type pageData struct {
	Title  string
	Active string
}
