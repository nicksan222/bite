package routes

import "github.com/gofiber/fiber/v3"

// pageChat handles GET / — chat is the default landing surface. The
// page bootstraps static/chat.js, which drives /api/chat over SSE and
// renders deltas into the transcript.
func pageChat() fiber.Handler {
	return func(c fiber.Ctx) error {
		return render(c, "chat.html", pageData{Title: "Chat", Active: "chat"})
	}
}
