package routes

import "github.com/gofiber/fiber/v3"

// pageMeals handles GET /meals. The form posts to /htmx/tool/log_meal
// and the right-hand card auto-loads /htmx/tool/meals_today every 30s
// (both wired in the template, no per-page JS).
func pageMeals() fiber.Handler {
	return func(c fiber.Ctx) error {
		return render(c, "meals.html", pageData{Title: "Meals", Active: "meals"})
	}
}
