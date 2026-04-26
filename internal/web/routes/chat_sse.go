package routes

import (
	"bufio"
	"cmp"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/nicksan222/bite/internal/ai"
)

// SSE event names shared by the server (writeSSE calls below) and the
// client (sse-swap / sse-close attributes in chatTurnTmpl). Centralising
// the strings here means a rename is a one-line refactor instead of a
// hunt across the package.
const (
	sseEventDelta = "delta"
	sseEventDone  = "done"
	sseEventError = "error"
)

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
			writeSSE(w, sseEventError, ev.Err.Error())
			return ""
		case ev.Done:
			writeSSE(w, sseEventDone, "")
			return cmp.Or(ev.Final, assembled.String())
		case ev.Delta != "":
			assembled.WriteString(ev.Delta)
			writeSSE(w, sseEventDelta, ev.Delta)
			if err := w.Flush(); err != nil {
				return ""
			}
		}
	}
	// Channel closed without a terminal Done event — return whatever we
	// accumulated so the session history still gets the partial reply.
	return assembled.String()
}

// writeSSEErrorAndDone sends an error event followed by a terminating
// done event, so the asst bubble's sse-close="done" hook still fires
// and the EventSource shuts cleanly.
func writeSSEErrorAndDone(w *bufio.Writer, msg string) {
	writeSSE(w, sseEventError, msg)
	writeSSE(w, sseEventDone, "")
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
