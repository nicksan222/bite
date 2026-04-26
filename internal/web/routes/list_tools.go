package routes

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// listTools handles GET /api/tools. Returns the registry-derived
// metadata the frontend needs to render generic tool forms.
func listTools(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d.ListTools == nil {
			return jsonError(c, http.StatusServiceUnavailable, "tool listing not configured")
		}
		return c.JSON(fiber.Map{"tools": d.ListTools()})
	}
}
