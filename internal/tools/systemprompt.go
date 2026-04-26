package tools

import "strings"

// RenderAppendix returns an "## Available tools" block listing every
// registered tool plus its per-tool system-prompt fragment. Append the
// result to the existing system prompt so the model has explicit guidance
// for *when* to call each tool.
//
// The structured tool spec still flows via ai.WithTools — this prompt list
// is the natural-language counterpart for tool selection.
func RenderAppendix() string {
	all := All()
	if len(all) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## Available tools\n\n")
	b.WriteString("You have access to the following tools. Each is also available to the user as a CLI subcommand and as a /<name> slash command in chat.\n")
	for _, t := range all {
		b.WriteString("\n### ")
		b.WriteString(t.Name)
		b.WriteString("\n")
		b.WriteString(t.Summary)
		b.WriteString("\n")
		body := t.Prompt
		if body == "" {
			body = t.Description
		}
		body = strings.TrimSpace(body)
		if body != "" && body != strings.TrimSpace(t.Summary) {
			b.WriteString("\n")
			b.WriteString(body)
			b.WriteString("\n")
		}
	}
	return b.String()
}
