package routes

import (
	"bytes"
	"errors"
	"html/template"
	"net/http"

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
			return htmlError(c, http.StatusServiceUnavailable, "tool invocation not configured")
		}
		name := c.Params("name")
		raw := mergeArgs(c.Queries(), formArgs(c))

		res, err := d.InvokeTool(c.Context(), name, raw)
		if err != nil {
			var nf NotFoundError
			if errors.As(err, &nf) {
				return htmlError(c, http.StatusNotFound, "tool not found: "+name)
			}
			return htmlError(c, http.StatusBadRequest, err.Error())
		}
		return c.Type("html").SendString(renderResultHTML(res))
	}
}

// alertTmpl wraps a message in daisyUI's alert-error component. Routed
// through html/template so escape behavior matches resultTmpl below —
// one escape primitive across the whole HTMX surface.
var alertTmpl = template.Must(template.New("alert").Parse(
	`<div role="alert" class="alert alert-error"><span>{{.}}</span></div>`,
))

// htmlAlert centralises the error-branch markup so the handler can't
// drift between branches.
func htmlAlert(msg string) string {
	var buf bytes.Buffer
	_ = alertTmpl.Execute(&buf, msg)
	return buf.String()
}

// htmlError sends an HTML alert fragment with the given status. Wraps
// the htmlAlert + Status + Type triple so the handler stays a flat list
// of intent rather than a chain of side-effects.
func htmlError(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).Type("html").SendString(htmlAlert(msg))
}

// resultTmpl renders a tool Result as an HTMX fragment. html/template
// auto-escapes every {{ .X }} interpolation in HTML context, so adding a
// new field cannot accidentally introduce an XSS hole. Tailwind / daisyUI
// classes keep the fragment visually consistent with the dashboard.
var resultTmpl = template.Must(template.New("result").Parse(
	`<div>` +
		`{{- if .Text -}}` +
		`<pre class="font-mono text-sm whitespace-pre-wrap leading-relaxed">{{.Text}}</pre>` +
		`{{- end -}}` +
		`{{- with .Table -}}` +
		`{{- if .Headers -}}` +
		`<div class="overflow-x-auto mt-2"><table class="table table-sm">` +
		`<thead><tr>{{range .Headers}}<th>{{.}}</th>{{end}}</tr></thead>` +
		`<tbody>{{range .Rows}}<tr>{{range .}}<td>{{.}}</td>{{end}}</tr>{{end}}</tbody>` +
		`{{- if .Footer -}}` +
		`<tfoot><tr>{{range .Footer}}<td>{{.}}</td>{{end}}</tr></tfoot>` +
		`{{- end -}}` +
		`</table></div>` +
		`{{- end -}}` +
		`{{- end -}}` +
		`</div>`,
))

// renderResultHTML produces the HTML fragment HTMX swaps in.
func renderResultHTML(r Result) string {
	var buf bytes.Buffer
	// Execution can only fail on writer errors; bytes.Buffer never errors.
	_ = resultTmpl.Execute(&buf, r)
	return buf.String()
}
