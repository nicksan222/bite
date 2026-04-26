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
// the assistant's final text on a clean Done so the caller can append
// it to the chat session; returns "" on a mid-stream error, on a
// client-disconnect (Flush failure), or when the channel closes
// without a Done event — i.e. any path the session shouldn't record.
//
// Every termination path that still has a live wire emits a terminating
// `event: done` (preceded by `event: error` for failure cases), so the
// browser's sse-close="done" hook fires cleanly. The exception is the
// client-disconnect path: the writer is already broken, so emitting
// more is pointless.
//
// The deferred Flush guarantees the terminal events reach the wire —
// without it we would rely on the SendStreamWriter caller flushing on
// closure return, which is true today but an implementation detail of
// fiber.
func pumpStreamEvents(w *bufio.Writer, events <-chan ai.StreamEvent) string {
	defer func() { _ = w.Flush() }()
	var assembled strings.Builder
	for ev := range events {
		switch {
		case ev.Err != nil:
			// Mid-stream failure: same wire shape as pre-stream
			// failures so the browser's sse-close="done" hook fires
			// cleanly regardless of where in the pipeline we failed.
			writeSSEErrorAndDone(w, ev.Err.Error())
			return ""
		case ev.Done:
			writeSSEDone(w)
			return cmp.Or(ev.Final, assembled.String())
		case ev.Delta != "":
			assembled.WriteString(ev.Delta)
			writeSSE(w, sseEventDelta, ev.Delta)
			if err := w.Flush(); err != nil {
				return ""
			}
		}
	}
	// Channel closed without a terminal Done event — emit one ourselves
	// so the browser's sse-close="done" hook still fires. Return whatever
	// we accumulated so the session history records the partial reply.
	writeSSEDone(w)
	return assembled.String()
}

// writeSSEDone sends the terminating done event that triggers the asst
// bubble's sse-close="done" hook on the client.
func writeSSEDone(w *bufio.Writer) {
	writeSSE(w, sseEventDone, "")
}

// writeSSEErrorAndDone sends an error event followed by a terminating
// done event, so the asst bubble's sse-close="done" hook still fires
// and the EventSource shuts cleanly.
func writeSSEErrorAndDone(w *bufio.Writer, msg string) {
	writeSSE(w, sseEventError, msg)
	writeSSEDone(w)
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
