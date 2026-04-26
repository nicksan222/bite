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
	{Tool: "meals_today", Title: "Today", Desc: "Everything logged today, with macro totals."},
	{Tool: "meals_week", Title: "This week", Desc: "Last 7 days of meals — quick scan for patterns."},
	{Tool: "streak", Title: "Streak", Desc: "How many days in a row you've logged at least one meal."},
	{Tool: "get_goals", Title: "Goals", Desc: "Calorie and macro targets currently configured."},
}

// pageDashboard handles GET /dashboard. The cards' bodies populate via
// HTMX (hx-trigger="load") — the server only emits the layout shell here.
func pageDashboard() fiber.Handler {
	return func(c fiber.Ctx) error {
		return render(c, "dashboard.html", struct {
			pageData
			Cards []dashboardCard
		}{
			pageData: pageData{Title: "Dashboard", Active: "dashboard"},
			Cards:    dashboardCards,
		})
	}
}
