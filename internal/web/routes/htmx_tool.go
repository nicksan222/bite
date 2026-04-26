package routes

import (
	"errors"
	"html"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// htmxTool handles GET and POST /htmx/tool/:name. Query parameters
// (GET) or form fields (POST) are forwarded as raw args so an HTMX
// form can drop directly onto a tool:
//
//	<form hx-post="/htmx/tool/log_meal" hx-target="#out">…</form>
//
// The response is an HTML fragment ready for hx-swap.
func htmxTool(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d.InvokeTool == nil {
			return jsonError(c, http.StatusServiceUnavailable, "tool invocation not configured")
		}
		name := c.Params("name")
		if name == "" {
			return jsonError(c, http.StatusBadRequest, "missing tool name")
		}

		raw := mergeArgs(c.Queries(), formArgs(c))

		res, err := d.InvokeTool(c.Context(), name, raw)
		if err != nil {
			var nf NotFoundError
			if errors.As(err, &nf) {
				return c.Status(http.StatusNotFound).
					Type("html").
					SendString(`<div class="bite-error">tool not found: ` + html.EscapeString(name) + `</div>`)
			}
			return c.Status(http.StatusBadRequest).
				Type("html").
				SendString(`<div class="bite-error">` + html.EscapeString(err.Error()) + `</div>`)
		}
		return c.Type("html").SendString(renderResultHTML(res))
	}
}

// renderResultHTML produces the HTML fragment HTMX swaps in. Hand-rolled
// (no template) because the structure is small, escape behavior is
// load-bearing, and html.EscapeString is the single primitive.
func renderResultHTML(r Result) string {
	var b strings.Builder
	b.WriteString(`<div class="bite-result">`)
	if r.Text != "" {
		b.WriteString(`<pre class="bite-result__text">`)
		b.WriteString(html.EscapeString(r.Text))
		b.WriteString(`</pre>`)
	}
	if r.Table != nil && len(r.Table.Headers) > 0 {
		b.WriteString(`<table class="bite-result__table"><thead><tr>`)
		for _, h := range r.Table.Headers {
			b.WriteString(`<th>`)
			b.WriteString(html.EscapeString(h))
			b.WriteString(`</th>`)
		}
		b.WriteString(`</tr></thead><tbody>`)
		for _, row := range r.Table.Rows {
			b.WriteString(`<tr>`)
			for _, cell := range row {
				b.WriteString(`<td>`)
				b.WriteString(html.EscapeString(cell))
				b.WriteString(`</td>`)
			}
			b.WriteString(`</tr>`)
		}
		b.WriteString(`</tbody>`)
		if len(r.Table.Footer) > 0 {
			b.WriteString(`<tfoot><tr>`)
			for _, cell := range r.Table.Footer {
				b.WriteString(`<td>`)
				b.WriteString(html.EscapeString(cell))
				b.WriteString(`</td>`)
			}
			b.WriteString(`</tr></tfoot>`)
		}
		b.WriteString(`</table>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
