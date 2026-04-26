package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nicksan222/bite/internal/ai"
)

func init() {
	Register(Tool{
		Name:    "ask",
		Summary: "Ask a one-shot question and stream the reply.",
		Description: `Send a single prompt to the model and return the full reply. Attach images
with --image (repeatable, local paths or http(s) URLs).`,
		Prompt: `Use ask sparingly — it bypasses chat memory and tools. Prefer normal chat
turns for anything that needs context, tool calls, or follow-ups.`,
		Examples: []Example{
			{Cmd: `bite ask "kcal in 200g salmon?"`, Desc: "one-shot question"},
		},
		Params: []Param{
			// prompt is positional but NOT Required: an image-only invocation
			// ("bite ask --image meal.jpg") is valid; runAsk rejects the case
			// where both prompt and image are absent.
			{Name: "prompt", Type: ParamString, Positional: true,
				Desc: "The question or instruction to send."},
			{Name: "image", Type: ParamStringList,
				Desc: "Image to attach (path or http(s) URL). Repeat for multiple."},
			{Name: "system", Type: ParamString,
				Desc: "Override the system prompt for this turn."},
		},
		Run: runAsk,
	})
}

func runAsk(ctx context.Context, deps Deps, args Args) (Result, error) {
	if err := deps.RequireAI(); err != nil {
		return Result{}, err
	}
	prompt := args.String("prompt")
	images := args.StringList("image")
	if prompt == "" && len(images) == 0 {
		return Result{}, errors.New("nothing to send — pass a prompt or --image")
	}

	msg := ai.Message{Role: ai.RoleUser, Content: prompt}
	for _, img := range images {
		msg.Attachments = append(msg.Attachments, asImageAttachment(img))
	}

	var streamOpts []ai.StreamOption
	if args.Has("system") {
		streamOpts = append(streamOpts, ai.WithSystemPrompt(args.String("system")))
	}

	ch, err := deps.AI.Stream(ctx, []ai.Message{msg}, streamOpts...)
	if err != nil {
		return Result{}, fmt.Errorf("stream: %w", err)
	}

	var sb strings.Builder
	for ev := range ch {
		switch {
		case ev.Err != nil:
			return Result{}, ev.Err
		case ev.Done:
			if ev.Final != "" && sb.Len() == 0 {
				sb.WriteString(ev.Final)
			}
			if deps.StreamWriter != nil {
				fmt.Fprintln(deps.StreamWriter)
			}
			return Result{Text: sb.String()}, nil
		default:
			if deps.StreamWriter != nil {
				fmt.Fprint(deps.StreamWriter, ev.Delta)
			}
			sb.WriteString(ev.Delta)
		}
	}
	return Result{Text: sb.String()}, nil
}

func asImageAttachment(v string) ai.Attachment {
	if isHTTPURL(v) {
		return ai.Attachment{URL: v}
	}
	return ai.Attachment{Path: v}
}

// isHTTPURL reports whether s starts with http:// or https://. Shared by
// ask (image attachments) and analyze_meal (media routing) so they apply
// the same URL-vs-path heuristic — `bite ask --image foo.jpg` and `bite
// analyze_meal --file foo.jpg` should classify a string the same way.
func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
