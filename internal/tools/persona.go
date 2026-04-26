package tools

import "strings"

// DefaultPersona is the bite chat persona injected when the user has not
// overridden BITE_SYSTEM_PROMPT. Keep this short and behavioural — specific
// tool guidance is auto-generated from each Tool's Prompt field by
// RenderAppendix(), so this prose only sets posture and house style.
const DefaultPersona = `You are bite, a friendly terminal AI nutritionist.
Help users track meals, understand calories and macros, and make healthy choices.
When the user reports eating something — even casually — call the appropriate logging tool to save it automatically.
When the user asks about intake, progress, goals, streaks, or wants to change a target, prefer calling tools over guessing.
Keep responses concise and scannable; this is a terminal, not a web app.`

// BuildSystemPrompt assembles the full system prompt: a custom override (or
// DefaultPersona if empty) followed by the auto-generated tool appendix.
// This is the single function every entry point should call instead of
// concatenating prompt strings by hand.
func BuildSystemPrompt(custom string) string {
	base := strings.TrimRight(custom, "\n")
	if base == "" {
		base = DefaultPersona
	}
	return base + RenderAppendix()
}
