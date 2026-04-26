package routes

import "github.com/gofiber/fiber/v3"

// dashboardCard is one tile on the home page. Static config — the live
// data is fetched by HTMX on load.
type dashboardCard struct {
	Tool  string
	Title string
	Desc  string
}

var dashboardCards = []dashboardCard{
	{"meals_today", "Today", "Everything logged today, with macro totals."},
	{"meals_week", "This week", "Last 7 days of meals — quick scan for patterns."},
	{"streak", "Streak", "How many days in a row you've logged at least one meal."},
	{"get_goals", "Goals", "Calorie and macro targets currently configured."},
}

// pageDashboard handles GET /dashboard. The cards' bodies populate via
// HTMX (hx-trigger="load") — the server only emits the layout shell here.
func pageDashboard() fiber.Handler {
	return func(c fiber.Ctx) error {
		return render(c, "dashboard.html", struct {
			pageData
			Cards []dashboardCard
		}{pageData{"Dashboard", "dashboard"}, dashboardCards})
	}
}
