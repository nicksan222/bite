package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
)

func init() {
	cmd := &cobra.Command{
		Use:   "analyze [MEDIA...]",
		Short: "Estimate calories + macros from any mix of media + text",
		Long: `Sends any combination of images, video, audio, and text to a single
analysis pass and prints a structured nutrition estimate.

Detection is by extension: .jpg/.png/.webp → image, .mp3/.wav/.m4a → audio
(Whisper-transcribed; needs OPENAI_API_KEY), .mp4/.mov → video (keyframes
via ffmpeg). Images can also be http(s) URLs.

Multiple files are treated as views of the SAME meal.`,
		Example: `  bite analyze meal.jpg
  bite analyze plate.jpg voice-note.m4a -m "had this for lunch"
  bite analyze cooking.mp4
  bite analyze https://example.com/food.png -m "is this healthy?"`,
		RunE: runAnalyze,
	}
	cmd.Flags().StringP("message", "m", "", "extra free-form context to include")
	cmd.Flags().BoolP("log", "l", false, "save the analysis result to the meal log")
	rootCmd.AddCommand(cmd)
}

var (
	analysisTitle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#9ECE6A"))
	analysisStat  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7AA2F7"))
	analysisMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("#565F89")).Italic(true)
)

func runAnalyze(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client, err := openAIClient(ctx, cfg)
	if err != nil {
		return err
	}

	var store db.Storer
	if log, _ := cmd.Flags().GetBool("log"); log {
		s, err := openStore(ctx, cfg)
		if err != nil {
			return err
		}
		defer s.Close()
		store = s
	}

	text, _ := cmd.Flags().GetString("message")
	return analyzeAndPrint(ctx, cmd, client, cfg.OpenAIAPIKey, text, args, store)
}

// analyzeAndPrint builds the AnalyzeInput, calls AnalyzeMeal, prints results,
// and optionally saves to the meal log when store is non-nil.
func analyzeAndPrint(ctx context.Context, cmd *cobra.Command, client ai.Streamer, openAIKey, text string, args []string, store db.Storer) error {
	if len(args) == 0 && text == "" {
		return fmt.Errorf("nothing to analyze — pass media files/URLs or use -m")
	}

	in := ai.AnalyzeInput{
		Text:           text,
		OpenAIAPIKey:   openAIKey,
		FramesPerVideo: 4,
	}
	for _, a := range args {
		if strings.HasPrefix(a, "http://") || strings.HasPrefix(a, "https://") {
			in.MediaURLs = append(in.MediaURLs, a)
		} else {
			in.MediaPaths = append(in.MediaPaths, a)
		}
	}

	analysis, err := ai.AnalyzeMeal(ctx, client, in)
	if err != nil {
		return err
	}

	if store != nil {
		if _, err := store.SaveMeal(ctx, db.MealInput{
			Title:       analysis.Title,
			Description: analysis.Description,
			Items:       analysis.Items,
			Kcal:        analysis.Kcal,
			ProteinG:    analysis.ProteinG,
			CarbsG:      analysis.CarbsG,
			FatG:        analysis.FatG,
		}); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not save to meal log: %v\n", err)
		}
	}

	return printAnalysis(cmd, analysis)
}

func printAnalysis(cmd *cobra.Command, a *ai.MealAnalysis) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, analysisTitle.Render(a.Title))
	if a.Description != "" {
		fmt.Fprintf(out, "%s\n\n", a.Description)
	}
	fmt.Fprintln(out, analysisStat.Render(fmt.Sprintf(
		"%.0f kcal · P %.0fg · C %.0fg · F %.0fg",
		a.Kcal, a.ProteinG, a.CarbsG, a.FatG,
	)))
	if len(a.Items) > 0 {
		fmt.Fprintln(out, analysisMuted.Render("items: "+strings.Join(a.Items, ", ")))
	}
	if a.Confidence != "" {
		fmt.Fprintln(out, analysisMuted.Render("confidence: "+a.Confidence))
	}
	return nil
}
