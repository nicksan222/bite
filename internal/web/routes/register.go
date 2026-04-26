package routes

import (
	"github.com/gofiber/fiber/v3"
)

// Register wires every endpoint onto app. The route map is the file
// map: each handler imported here lives in its own file under this
// package. Adding a route means dropping a file and adding one line
// here.
func Register(app *fiber.App, d Deps) {
	api := app.Group("/api")
	api.Get("/tools", listTools(d))
	api.Post("/tools/:name", invokeTool(d))
	api.Post("/chat", chatStream(d))

	htmx := app.Group("/htmx")
	htmx.Get("/tool/:name", htmxTool(d))
	htmx.Post("/tool/:name", htmxTool(d))

	app.Get("/static/*", serveStatic())

	app.Get("/", pageDashboard())
	app.Get("/chat", pageChat())
	app.Get("/meals", pageMeals())
	app.Get("/tools", pageTools(d))
}
