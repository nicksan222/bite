package routes

import (
	"bytes"
	"html/template"
)

// chatTurnTmpl is the HTML fragment hx-swapped into the transcript when
// the user submits a turn. The user bubble is fully formed; the
// assistant bubble has sse-connect bound, htmx-ext-sse appends each
// "delta" event into .chat-bubble (sse-swap="delta" + hx-swap="beforeend"),
// and the bubble's sse-close="done" attribute tells htmx-ext-sse to
// shut the EventSource the moment a "done" event arrives. The
// alert div catches "error" events for the same connection. The
// session cookie carries history forward, so neither bubble needs
// hidden inputs.
var chatTurnTmpl = template.Must(template.New("chat-turn").Parse(
	`<div class="chat chat-end" data-role="user">
	<div class="chat-bubble chat-bubble-primary">{{.UserText}}</div>
</div>
<div class="chat chat-start" data-role="assistant"
     hx-ext="sse"
     sse-connect="` + chatStreamPathPrefix + `{{.TurnID}}"
     sse-close="done">
	<div class="chat-bubble empty:hidden" sse-swap="delta" hx-swap="beforeend"></div>
	<div class="alert alert-error empty:hidden mt-1" role="alert" sse-swap="error" hx-swap="textContent"></div>
</div>
`))

// chatTurnData is the template's expected shape. Named (rather than an
// inline anonymous struct) so the contract is discoverable: a future
// reader can grep for chatTurnData and find both the template fields
// and the renderer in one go.
type chatTurnData struct {
	TurnID   string
	UserText string
}

// renderChatTurn produces the HTML fragment hx-swapped into the
// transcript on a successful POST /api/chat.
func renderChatTurn(turnID, userText string) ([]byte, error) {
	var buf bytes.Buffer
	err := chatTurnTmpl.Execute(&buf, chatTurnData{TurnID: turnID, UserText: userText})
	return buf.Bytes(), err
}
