package routes

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/nicksan222/bite/internal/ai"
)

// chatRequest is the SSE chat input. Message is the user's new turn;
// History seeds the model with prior context. The endpoint is stateless,
// so every request replays enough prior turns to keep the model coherent.
type chatRequest struct {
	Message string       `json:"message"`
	History []chatMsgDTO `json:"history,omitempty"`
}

// chatMsgDTO is one prior turn in chatRequest.History. Role is
// validated against the user/assistant allowlist by parseChatRole; an
// untyped string here keeps the JSON decode lax and lets us produce a
// helpful error message instead of an opaque "cannot unmarshal" failure.
type chatMsgDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SSE event payloads — one struct per `event:` name. Typed shapes keep
// the wire contract visible at a glance and avoid a fresh map allocation
// per delta token.
type (
	sseDelta struct {
		Text string `json:"text"`
	}
	sseDone struct {
		Final string `json:"final"`
	}
	sseError struct {
		Message string `json:"message"`
	}
)

// chatStream handles POST /api/chat as Server-Sent Events. The browser
// reads one event per delta plus a terminating "done" event. Tool-calls
// happen transparently inside ai.Streamer because Deps.StreamOpts
// returns the ai.WithTools binding.
func chatStream(d Deps) fiber.Handler {
	return func(c fiber.Ctx) error {
		if d.AI == nil {
			return jsonError(c, http.StatusServiceUnavailable, "AI not configured (set ANTHROPIC_API_KEY)")
		}

		var req chatRequest
		if err := c.Bind().Body(&req); err != nil {
			return jsonError(c, http.StatusBadRequest, "invalid json: "+err.Error())
		}
		if strings.TrimSpace(req.Message) == "" {
			return jsonError(c, http.StatusBadRequest, "empty message")
		}

		history, err := buildChatHistory(req)
		if err != nil {
			return jsonError(c, http.StatusBadRequest, err.Error())
		}

		ctx := c.Context()
		var opts []ai.StreamOption
		if d.StreamOpts != nil {
			opts = d.StreamOpts()
		}
		events, err := d.AI.Stream(ctx, history, opts...)
		if err != nil {
			return jsonError(c, http.StatusBadGateway, err.Error())
		}

		setSSEHeaders(c)
		return c.SendStreamWriter(func(w *bufio.Writer) {
			pumpStreamEvents(w, events)
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
// when the channel closes naturally, on a terminal Done/Err event, or
// when a write to w fails (which means the client disconnected).
//
// The deferred Flush guarantees the terminal "done"/"error" event
// reaches the wire — without it we would rely on the
// SendStreamWriter caller flushing on closure return, which is true
// today but an implementation detail of fiber.
func pumpStreamEvents(w *bufio.Writer, events <-chan ai.StreamEvent) {
	defer func() { _ = w.Flush() }()
	for ev := range events {
		switch {
		case ev.Err != nil:
			writeSSE(w, "error", sseError{Message: ev.Err.Error()})
			return
		case ev.Done:
			writeSSE(w, "done", sseDone{Final: ev.Final})
			return
		case ev.Delta != "":
			writeSSE(w, "delta", sseDelta{Text: ev.Delta})
			if err := w.Flush(); err != nil {
				return
			}
		}
	}
}

// buildChatHistory turns the JSON request into the AI message slice.
// Only user/assistant roles are accepted from the wire — system/tool
// roles would let a direct caller seed the model with a fake persona,
// bypassing the appendix that tools/systemprompt builds.
func buildChatHistory(req chatRequest) ([]ai.Message, error) {
	out := make([]ai.Message, 0, len(req.History)+1)
	for i, m := range req.History {
		role, err := parseChatRole(m.Role)
		if err != nil {
			return nil, fmt.Errorf("history[%d]: %w", i, err)
		}
		out = append(out, ai.Message{Role: role, Content: m.Content})
	}
	out = append(out, ai.Message{Role: ai.RoleUser, Content: req.Message})
	return out, nil
}

// parseChatRole accepts only the two roles a real chat client produces.
// Anything else — empty, "system", a typo — is rejected up front so it
// never reaches the model.
func parseChatRole(s string) (ai.Role, error) {
	role := ai.Role(s)
	switch role {
	case ai.RoleUser, ai.RoleAssistant:
		return role, nil
	default:
		return "", fmt.Errorf("invalid role %q (want user or assistant)", s)
	}
}

// writeSSE emits one Server-Sent Event. Payload must be one of the
// sse* structs above — those are JSON-marshal-safe by construction, so
// the marshal error is suppressed. Write errors mean the client
// disconnected; the caller's loop terminates on the next iteration.
func writeSSE(w *bufio.Writer, event string, payload any) {
	body, _ := json.Marshal(payload)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
}
