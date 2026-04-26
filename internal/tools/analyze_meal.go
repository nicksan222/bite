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
		Name:    "analyze_meal",
		Summary: "Estimate calories and macros from media or text without saving.",
		Description: `Analyze any combination of images, video, audio, and text and return a
nutrition estimate without saving.

Detection is by extension: .jpg/.png/.webp → image, .mp3/.wav/.m4a → audio
(Whisper-transcribed; needs OPENAI_API_KEY), .mp4/.mov → video (keyframes
via ffmpeg). http(s) URLs are treated as remote images. Multiple files are
treated as views of the SAME meal.`,
		Prompt: `Use analyze_meal when the user wants a "what would this be?" estimate from media
or text without committing it to the diary. If the user clearly wants the meal
saved, prefer log_meal_from_media instead.`,
		Examples: []Example{
			{Cmd: "bite analyze_meal --file plate.jpg", Desc: "estimate from media without saving"},
		},
		Params: []Param{
			{Name: "text", Type: ParamString, Positional: true,
				Desc: "Free-form text describing the meal."},
			{Name: "file", Type: ParamStringList,
				Desc: "Media file path(s) or URL(s). Repeat the flag for multiple."},
		},
		Run: runAnalyzeMeal,
	})
}

func runAnalyzeMeal(ctx context.Context, deps Deps, args Args) (Result, error) {
	analysis, err := analyzeFromArgs(ctx, deps, args)
	if err != nil {
		return Result{}, err
	}
	return Result{Text: formatAnalysis(analysis)}, nil
}

// analyzeFromArgs is shared by analyze_meal and log_meal_from_media.
func analyzeFromArgs(ctx context.Context, deps Deps, args Args) (*ai.MealAnalysis, error) {
	if err := deps.RequireAI(); err != nil {
		return nil, err
	}
	text := args.String("text")
	files := args.StringList("file")
	if text == "" && len(files) == 0 {
		return nil, errors.New("nothing to analyze — pass text or --file")
	}
	in := ai.AnalyzeInput{
		Text:           text,
		OpenAIAPIKey:   deps.OpenAIAPIKey,
		FramesPerVideo: 4,
	}
	for _, f := range files {
		if isHTTPURL(f) {
			in.MediaURLs = append(in.MediaURLs, f)
		} else {
			in.MediaPaths = append(in.MediaPaths, f)
		}
	}
	a, err := ai.AnalyzeMeal(ctx, deps.AI, in)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	return a, nil
}

func formatAnalysis(a *ai.MealAnalysis) string {
	var sb strings.Builder
	sb.WriteString(a.Title)
	if a.Description != "" {
		sb.WriteString("\n")
		sb.WriteString(a.Description)
	}
	fmt.Fprintf(&sb, "\n%.0f kcal · P %.0fg · C %.0fg · F %.0fg",
		a.Kcal, a.ProteinG, a.CarbsG, a.FatG)
	if len(a.Items) > 0 {
		fmt.Fprintf(&sb, "\nitems: %s", strings.Join(a.Items, ", "))
	}
	if a.Confidence != "" {
		fmt.Fprintf(&sb, "\nconfidence: %s", a.Confidence)
	}
	return sb.String()
}
