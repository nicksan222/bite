package tools

import (
	"context"
	"fmt"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	Register(Tool{
		Name:    "log_meal_from_media",
		Summary: "Analyze media or text, then log the estimated meal.",
		Description: `Analyze any combination of images, video, audio, and text and save the
result to the meal log. Equivalent to running analyze_meal followed by
log_meal in one step.`,
		Prompt: `Use log_meal_from_media when the user shares a photo, voice note, or video of
a meal and wants it logged. Prefer log_meal for plain-text descriptions.`,
		Examples: []Example{
			{Cmd: `bite log_meal_from_media "lunch" --file plate.jpg`, Desc: "estimate from media + save"},
		},
		Params: []Param{
			{Name: "text", Type: ParamString, Positional: true,
				Desc: "Free-form text describing the meal."},
			{Name: "file", Type: ParamStringList,
				Desc: "Media file path(s) or URL(s). Repeat the flag for multiple."},
		},
		Run: runLogMealFromMedia,
	})
}

func runLogMealFromMedia(ctx context.Context, deps Deps, args Args) (Result, error) {
	a, err := analyzeFromArgs(ctx, deps, args)
	if err != nil {
		return Result{}, err
	}
	meal, err := deps.Store.SaveMeal(ctx, db.MealInput{
		Title:       a.Title,
		Description: a.Description,
		Items:       a.Items,
		Kcal:        a.Kcal,
		ProteinG:    a.ProteinG,
		CarbsG:      a.CarbsG,
		FatG:        a.FatG,
		EatenAt:     deps.NowOrDefault(),
	})
	if err != nil {
		return Result{}, fmt.Errorf("save: %w", err)
	}
	return Result{Text: fmt.Sprintf(
		"logged %s (#%d) — %.0f kcal, P %.0fg, C %.0fg, F %.0fg",
		meal.Title, meal.ID, meal.Kcal, meal.ProteinG, meal.CarbsG, meal.FatG,
	)}, nil
}
