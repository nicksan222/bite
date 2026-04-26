package routes

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
)

// serveStatic returns a handler for /static/*. MaxAge stays 0 so live
// reload (`make web-dev`) actually shows new CSS/JS without forcing a
// hard refresh — assets are tiny and embedded, so revalidation is cheap.
// ByteRange enables progressive loading for the larger htmx.min.js.
func serveStatic() fiber.Handler {
	return static.New("", static.Config{
		FS:        staticFS,
		ByteRange: true,
	})
}
