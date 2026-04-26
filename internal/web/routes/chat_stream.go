package routes

import (
	"bufio"
	"cmp"
	"context"
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

		html, err := renderChatTurn(turnID, message)
		if err != nil {
			// Defensive: chatTurnTmpl parses at init and the data
			// shape is fixed, so this branch is unreachable in
			// practice — but if it ever fires, the form expects HTML
			// and would render a JSON envelope as text.
			return htmlError(c, http.StatusInternalServerError, "render chat turn: "+err.Error())
		}
		return c.Type("html").Send(html)
	}
}

// chatStream handles GET /api/chat/stream/:id — the SSE endpoint
// htmx-ext-sse opens after a successful POST /api/chat. The response is
// ALWAYS an SSE 200 stream so a failure surfaces as an `event: error`
// the asst bubble can swap, rather than as a non-200 EventSource that
// htmx-ext-sse would silently drop into a stuck spinner. Events:
//
//	event: delta\ndata: <token>\n\n
//	event: done\ndata: \n\n
//	event: error\ndata: <message>\n\n
//
// Plain text (not JSON) keeps htmx-ext-sse's sse-swap a one-liner: the
// extension drops event.data straight into the target. On a clean
// terminate the assistant message is appended to the session history so
// the next turn carries proper context.
func chatStream(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		// Capture every per-request value here. SendStreamWriter runs
		// the callback in a goroutine that outlives the handler return,
		// so reading from c (params, context, etc.) after this point
		// races fiber's ctx recycling and panics.
		turnID := c.Params("id")
		ctx := c.Context()
		setSSEHeaders(c)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			runChatStream(ctx, d, turnID, w)
		})
	}
}

// runChatStream is chatStream's SSE-body driver. Split out so every
// failure path can write an `event: error\nevent: done` pair through
// the same writer instead of branching between JSON and SSE responses.
func runChatStream(ctx context.Context, d Deps, turnID string, w *bufio.Writer) {
	if d.AI == nil {
		writeSSEErrorAndDone(w, "AI not configured (set ANTHROPIC_API_KEY)")
		return
	}
	turn, ok := turnStore.pop(turnID)
	if !ok {
		writeSSEErrorAndDone(w, "turn expired or not found")
		return
	}

	var opts []ai.StreamOption
	if d.StreamOpts != nil {
		opts = d.StreamOpts()
	}
	events, err := d.AI.Stream(ctx, turn.history, opts...)
	if err != nil {
		writeSSEErrorAndDone(w, err.Error())
		return
	}

	final := pumpStreamEvents(w, events)
	if final != "" {
		chatSessionStore.appendTurn(turn.sessionID, turn.userMsg, final)
	}
}

// writeSSEErrorAndDone sends an error event followed by a terminating
// done event, so the asst bubble's sse-close="done" hook still fires
// and the EventSource shuts cleanly.
func writeSSEErrorAndDone(w *bufio.Writer, msg string) {
	writeSSE(w, "error", msg)
	writeSSE(w, "done", "")
}

// setSSEHeaders configures the response for a Server-Sent Events stream
// — Content-Type plus the cache/buffer hints that keep proxies and
// browsers from delaying or coalescing per-token deltas.
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
			writeSSE(w, "done", "")
			return cmp.Or(ev.Final, assembled.String())
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
// newline still parses on the client. Empty bodies emit a single
// `data: ` line, which the SSE spec accepts as zero-length data.
func writeSSE(w *bufio.Writer, event, body string) {
	fmt.Fprintf(w, "event: %s\n", event)
	for _, line := range strings.Split(body, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	fmt.Fprint(w, "\n")
}
