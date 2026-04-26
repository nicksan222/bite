package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
)

func init() {
	cmd := &cobra.Command{
		Use:   "log [DESCRIPTION]",
		Short: "Analyze a meal and save it to your log",
		Long: `Describe a meal in natural language and bite will estimate its nutrition
and save it to your local meal log.

Examples:
  bite log "200g pasta with pesto sauce"
  bite log "grilled chicken breast with salad" -f meal.jpg`,
		Args: cobra.MinimumNArgs(1),
		RunE: runLog,
	}
	cmd.Flags().StringArrayP("file", "f", nil, "media files or URLs to include")
	rootCmd.AddCommand(cmd)
}

func runLog(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	client, err := openAIClient(ctx, cfg)
	if err != nil {
		return err
	}
	store, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()

	files, _ := cmd.Flags().GetStringArray("file")
	return logMeal(ctx, cmd.OutOrStdout(), client, store, strings.Join(args, " "), files, cfg.OpenAIAPIKey)
}

// logMeal analyzes a meal description and saves it to the store.
func logMeal(ctx context.Context, out io.Writer, client ai.Streamer, store db.Storer, text string, files []string, openAIKey string) error {
	in := ai.AnalyzeInput{
		Text:           text,
		OpenAIAPIKey:   openAIKey,
		FramesPerVideo: 4,
	}
	for _, f := range files {
		if strings.HasPrefix(f, "http://") || strings.HasPrefix(f, "https://") {
			in.MediaURLs = append(in.MediaURLs, f)
		} else {
			in.MediaPaths = append(in.MediaPaths, f)
		}
	}

	analysis, err := ai.AnalyzeMeal(ctx, client, in)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	meal, err := store.SaveMeal(ctx, db.MealInput{
		Title:       analysis.Title,
		Description: analysis.Description,
		Items:       analysis.Items,
		Kcal:        analysis.Kcal,
		ProteinG:    analysis.ProteinG,
		CarbsG:      analysis.CarbsG,
		FatG:        analysis.FatG,
	})
	if err != nil {
		return fmt.Errorf("save: %w", err)
	}

	fmt.Fprintf(out, "logged %s (#%d) — %.0f kcal, P %.0fg, C %.0fg, F %.0fg\n",
		meal.Title, meal.ID, meal.Kcal, meal.ProteinG, meal.CarbsG, meal.FatG)
	return nil
}
