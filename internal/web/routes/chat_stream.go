package routes

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/nicksan222/bite/internal/ai"
)

// chatStart handles POST /api/chat. The dashboard's chat form posts to
// it via hx-post. The handler:
//
//   - resolves (or creates) a server-side chat session via cookie
//   - validates the new user message
//   - stashes a pending turn (history + user message + session)
//   - returns the HTML fragment containing both bubbles, with the
//     assistant bubble bound to /api/chat/stream/<id> via htmx-ext-sse
//
// The browser auto-opens the SSE connection and streams tokens into the
// assistant bubble. There is no JS in the page — htmx + the SSE
// extension drive everything.
func chatStart(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d.AI == nil {
			return htmlError(c, http.StatusServiceUnavailable, "AI not configured (set ANTHROPIC_API_KEY)")
		}
		message := strings.TrimSpace(c.FormValue("message"))
		if message == "" {
			return htmlError(c, http.StatusBadRequest, "empty message")
		}

		sessionID, history := chatSessionStore.ensure(c)
		history = append(history, ai.Message{Role: ai.RoleUser, Content: message})

		turnID := turnStore.stash(pendingTurn{
			sessionID: sessionID,
			history:   history,
			userMsg:   message,
		})

		var buf bytes.Buffer
		if err := chatTurnTmpl.Execute(&buf, struct {
			TurnID   string
			UserText string
		}{TurnID: turnID, UserText: message}); err != nil {
			return fiber.NewError(http.StatusInternalServerError, "render chat turn: "+err.Error())
		}
		return c.Type("html").Send(buf.Bytes())
	}
}

// chatStream handles GET /api/chat/stream/:id — the SSE endpoint
// htmx-ext-sse opens after a successful POST /api/chat. It pops the
// stashed turn, calls the model, and streams plain-text events:
//
//	event: delta\ndata: <token>\n\n
//	event: done\ndata:\n\n
//	event: error\ndata: <message>\n\n
//
// Plain text (not JSON) keeps htmx-ext-sse's sse-swap a one-liner: the
// extension drops event.data straight into the target. On a clean
// terminate the assistant message is appended to the session history so
// the next turn carries proper context.
func chatStream(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d.AI == nil {
			return jsonError(c, http.StatusServiceUnavailable, "AI not configured (set ANTHROPIC_API_KEY)")
		}
		turn, ok := turnStore.pop(c.Params("id"))
		if !ok {
			return jsonError(c, http.StatusNotFound, "turn expired or not found")
		}

		ctx := c.Context()
		var opts []ai.StreamOption
		if d.StreamOpts != nil {
			opts = d.StreamOpts()
		}
		events, err := d.AI.Stream(ctx, turn.history, opts...)
		if err != nil {
			return jsonError(c, http.StatusBadGateway, err.Error())
		}

		setSSEHeaders(c)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			final := pumpStreamEvents(w, events)
			if final != "" {
				chatSessionStore.appendTurn(turn.sessionID, turn.userMsg, final)
			}
		})
	}
}

// setSSEHeaders configures the response for a Server-Sent Events stream.
// Called only once Stream has succeeded so a JSON-error fallback above
// isn't preceded by SSE Content-Type being staged.
func setSSEHeaders(c fiber.Ctx) {
	c.Set("Content-Type", "text/event-stream")
	c.Set("Cache-Control", "no-cache")
	c.Set("Connection", "keep-alive")
	c.Set("X-Accel-Buffering", "no")
}

// pumpStreamEvents drains the model channel into SSE events. Returns
// the final assistant text (so the caller can append it to the chat
// session) — empty string if the stream errored or aborted before a
// done event.
//
// The deferred Flush guarantees the terminal "done"/"error" event
// reaches the wire — without it we would rely on the SendStreamWriter
// caller flushing on closure return, which is true today but an
// implementation detail of fiber.
func pumpStreamEvents(w *bufio.Writer, events <-chan ai.StreamEvent) string {
	defer func() { _ = w.Flush() }()
	var assembled strings.Builder
	for ev := range events {
		switch {
		case ev.Err != nil:
			writeSSE(w, "error", ev.Err.Error())
			return ""
		case ev.Done:
			final := ev.Final
			if final == "" {
				final = assembled.String()
			}
			writeSSE(w, "done", "")
			return final
		case ev.Delta != "":
			assembled.WriteString(ev.Delta)
			writeSSE(w, "delta", ev.Delta)
			if err := w.Flush(); err != nil {
				return ""
			}
		}
	}
	// Channel closed without a terminal Done event — return whatever we
	// accumulated so the session history still gets the partial reply.
	return assembled.String()
}

// writeSSE emits one Server-Sent Event with plain-text body. Multi-line
// bodies are split into separate `data:` lines so a token containing a
// newline still parses on the client.
func writeSSE(w *bufio.Writer, event, body string) {
	fmt.Fprintf(w, "event: %s\n", event)
	if body == "" {
		fmt.Fprint(w, "data:\n")
	} else {
		for _, line := range strings.Split(body, "\n") {
			fmt.Fprintf(w, "data: %s\n", line)
		}
	}
	fmt.Fprint(w, "\n")
}
