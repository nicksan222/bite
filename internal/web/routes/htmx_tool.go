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
				return c.Status(http.StatusNotFound).Type("html").SendString(htmlAlert("tool not found: " + name))
			}
			return c.Status(http.StatusBadRequest).Type("html").SendString(htmlAlert(err.Error()))
		}
		return c.Type("html").SendString(renderResultHTML(res))
	}
}

// htmlAlert wraps a message in daisyUI's alert-error component. The
// markup is small enough to inline; centralising means the handler's
// error branches can't drift from one another.
func htmlAlert(msg string) string {
	return `<div role="alert" class="alert alert-error"><span>` + html.EscapeString(msg) + `</span></div>`
}

// renderResultHTML produces the HTML fragment HTMX swaps in. Hand-rolled
// (no template) because the structure is small, escape behavior is
// load-bearing, and html.EscapeString is the single primitive. Uses
// Tailwind / daisyUI classes so the fragment renders consistently with
// the rest of the dashboard.
func renderResultHTML(r Result) string {
	var b strings.Builder
	b.WriteString(`<div>`)
	if r.Text != "" {
		b.WriteString(`<pre class="font-mono text-sm whitespace-pre-wrap leading-relaxed">`)
		b.WriteString(html.EscapeString(r.Text))
		b.WriteString(`</pre>`)
	}
	if r.Table != nil && len(r.Table.Headers) > 0 {
		b.WriteString(`<div class="overflow-x-auto mt-2"><table class="table table-sm"><thead><tr>`)
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
		b.WriteString(`</table></div>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}
