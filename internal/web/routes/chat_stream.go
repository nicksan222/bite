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

type chatMsgDTO struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

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

		// Headers stay deferred until Stream succeeds so the JSON-error
		// fallback above isn't shipped with SSE Content-Type already set.
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("X-Accel-Buffering", "no")

		return c.SendStreamWriter(func(w *bufio.Writer) {
			for ev := range events {
				switch {
				case ev.Err != nil:
					writeSSE(w, "error", map[string]string{"message": ev.Err.Error()})
					return
				case ev.Done:
					writeSSE(w, "done", map[string]string{"final": ev.Final})
					return
				case ev.Delta != "":
					writeSSE(w, "delta", map[string]string{"text": ev.Delta})
					if err := w.Flush(); err != nil {
						return
					}
				}
			}
		})
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
	switch ai.Role(s) {
	case ai.RoleUser, ai.RoleAssistant:
		return ai.Role(s), nil
	default:
		return "", fmt.Errorf("invalid role %q (want user or assistant)", s)
	}
}

// writeSSE emits one Server-Sent Event. Errors writing mean the client
// disconnected; the caller's loop ends on the next iteration.
func writeSSE(w *bufio.Writer, event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		body = []byte(`{"message":"encode failed"}`)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, body)
}
