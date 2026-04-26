package routes

import (
	"bytes"
	"html/template"
)

// chatTurnTmpl is the HTML fragment hx-swapped into the transcript when
// the user submits a turn. The user bubble is fully formed; the
// assistant bubble has sse-connect bound, and htmx-ext-sse appends each
// "delta" event into .chat-bubble. The "done" event closes the SSE
// connection. The session cookie carries history forward, so neither
// bubble needs hidden inputs.
var chatTurnTmpl = template.Must(template.New("chat-turn").Parse(
	`<div class="chat chat-end" data-role="user">
	<div class="chat-bubble chat-bubble-primary">{{.UserText}}</div>
</div>
<div class="chat chat-start" data-role="assistant"
     hx-ext="sse"
     sse-connect="` + chatStreamPathPrefix + `{{.TurnID}}"
     sse-close="done">
	<div class="chat-bubble" sse-swap="delta" hx-swap="beforeend"></div>
	<div class="alert alert-error empty:hidden mt-1" role="alert" sse-swap="error" hx-swap="textContent"></div>
</div>
`))

// renderChatTurn produces the HTML fragment hx-swapped into the
// transcript on a successful POST /api/chat. Encapsulating the
// anonymous-struct dance keeps the handler readable and gives the
// template's data shape one canonical home.
func renderChatTurn(turnID, userText string) ([]byte, error) {
	var buf bytes.Buffer
	err := chatTurnTmpl.Execute(&buf, struct {
		TurnID   string
		UserText string
	}{TurnID: turnID, UserText: userText})
	return buf.Bytes(), err
}
