package routes

import "github.com/gofiber/fiber/v3"

// pageChat handles GET / — chat is the default landing surface. The
// page is htmx-driven: hx-post sends the message to /api/chat, which
// returns the dual-bubble HTML scaffold; htmx-ext-sse then opens
// /api/chat/stream/<id> and streams tokens into the assistant bubble.
func pageChat() fiber.Handler {
	return func(c fiber.Ctx) error {
		return render(c, "chat.html", pageData{Title: "Chat", Active: "chat"})
	}
}
