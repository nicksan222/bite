package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// serveStatic returns a handler for /static/*. ByteRange enables
// progressive loading of larger assets (htmx.min.js); MaxAge lets the
// browser cache for an hour, which is fine since asset filenames are
// content-stable for now.
func serveStatic() fiber.Handler {
	return static.New("", static.Config{
		FS:        staticFS,
		MaxAge:    3600,
		ByteRange: true,
	})
}
