package routes

import "github.com/gofiber/fiber/v3"

// pageTools handles GET /tools — the registry browser. Resolves the
// tool list per request so the rendered page reflects the current
// registry without a server restart.
func pageTools(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		var list []ToolMeta
		if d.ListTools != nil {
			list = d.ListTools()
		}
		return render(c, "tools.html", struct {
			pageData
			Tools []ToolMeta
		}{
			pageData: pageData{Title: "Tools", Active: "tools"},
			Tools:    list,
		})
	}
}
