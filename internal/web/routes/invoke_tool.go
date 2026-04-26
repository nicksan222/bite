package routes

import (
	"errors"
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// invokeTool handles POST /api/tools/:name. Body is a JSON object whose
// keys match the tool's Param names; values are forwarded as-is to
// InvokeTool. The decoded Result is returned as JSON.
func invokeTool(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d.InvokeTool == nil {
			return jsonError(c, http.StatusServiceUnavailable, "tool invocation not configured")
		}
		name := c.Params("name")
		raw := map[string]any{}
		if len(c.Body()) > 0 {
			if err := c.Bind().Body(&raw); err != nil {
				return jsonError(c, http.StatusBadRequest, "invalid json: "+err.Error())
			}
		}

		res, err := d.InvokeTool(c.Context(), name, raw)
		if err != nil {
			var nf NotFoundError
			if errors.As(err, &nf) {
				return jsonError(c, http.StatusNotFound, err.Error())
			}
			return jsonError(c, http.StatusBadRequest, err.Error())
		}
		return c.JSON(res)
	}
}
