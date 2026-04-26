package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"

	"github.com/nicksan222/bite/internal/ai"
)

// AITools renders every registered tool as an ai.Tool, ready to pass to
// ai.WithTools. Each ai.Tool's Execute closure parses JSON args, calls the
// underlying Run, and returns the rendered Result text.
func AITools(deps Deps) []ai.Tool {
	all := All()
	out := make([]ai.Tool, 0, len(all))
	for _, t := range all {
		out = append(out, toAITool(t, deps))
	}
	return out
}

func toAITool(t Tool, deps Deps) ai.Tool {
	return ai.Tool{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  paramsToSchema(t.Params),
		Execute: func(ctx context.Context, args string) (string, error) {
			raw := map[string]any{}
			if args != "" {
				if err := json.Unmarshal([]byte(args), &raw); err != nil {
					return "", fmt.Errorf("parse args: %w", err)
				}
			}
			res, err := t.Run(ctx, deps, NewArgs(raw))
			if err != nil {
				return "", err
			}
			return renderForAI(res), nil
		},
	}
}

// paramsToSchema converts our Param slice into eino's parameter map. Returns
// nil when there are no parameters so eino skips the schema entirely.
func paramsToSchema(ps []Param) map[string]*schema.ParameterInfo {
	if len(ps) == 0 {
		return nil
	}
	out := make(map[string]*schema.ParameterInfo, len(ps))
	for _, p := range ps {
		out[p.Name] = paramToSchema(p)
	}
	return out
}

func paramToSchema(p Param) *schema.ParameterInfo {
	info := &schema.ParameterInfo{
		Desc:     p.Desc,
		Required: p.Required,
	}
	switch p.Type {
	case ParamString:
		info.Type = schema.String
	case ParamInt:
		info.Type = schema.Integer
	case ParamFloat:
		info.Type = schema.Number
	case ParamBool:
		info.Type = schema.Boolean
	case ParamStringList:
		info.Type = schema.Array
		info.ElemInfo = &schema.ParameterInfo{Type: schema.String}
	}
	return info
}

// RenderResultForChat renders a Result into Markdown text suitable for chat
// transcripts (TUI history) and for the model. Same shape as renderForAI.
func RenderResultForChat(r Result) string { return renderForAI(r) }

// renderForAI produces the plain-text payload the model sees. Result.Text is
// the body; Result.Table (if any) is appended as a Markdown table.
func renderForAI(r Result) string {
	if r.Table == nil {
		return r.Text
	}
	body := r.Text
	if body != "" {
		body += "\n\n"
	}
	return body + renderMarkdownTable(r.Table)
}

func renderMarkdownTable(t *Table) string {
	if t == nil || len(t.Headers) == 0 {
		return ""
	}
	var b []byte
	b = append(b, '|')
	for _, h := range t.Headers {
		b = append(b, ' ')
		b = append(b, h...)
		b = append(b, ' ', '|')
	}
	b = append(b, '\n', '|')
	for range t.Headers {
		b = append(b, ' ', '-', '-', '-', ' ', '|')
	}
	b = append(b, '\n')
	for _, row := range t.Rows {
		b = append(b, '|')
		for _, c := range row {
			b = append(b, ' ')
			b = append(b, c...)
			b = append(b, ' ', '|')
		}
		b = append(b, '\n')
	}
	if len(t.Footer) > 0 {
		b = append(b, '|')
		for _, c := range t.Footer {
			b = append(b, ' ')
			b = append(b, c...)
			b = append(b, ' ', '|')
		}
		b = append(b, '\n')
	}
	return string(b)
}
