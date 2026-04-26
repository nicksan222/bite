package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"

	"github.com/nicksan222/bite/internal/ai"
	"github.com/nicksan222/bite/internal/db"
)

// makeChatTools returns the set of ai.Tool values wired into every chat session.
// Add new tools here; they are forwarded to ai.Client.Stream via ai.WithTools.
func makeChatTools(store db.Storer) []ai.Tool {
	return []ai.Tool{
		logMealTool(store),
		getMealsTodayTool(store),
		getMealsWeekTool(store),
		getGoalsTool(store),
		setGoalsTool(store),
		deleteMealTool(store),
		getStreakTool(store),
	}
}

// getStreakTool returns the current daily logging streak so the model can
// encourage and acknowledge consistent tracking habits.
func getStreakTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "get_streak",
		Description: `Return the number of consecutive days the user has logged at least one meal.
Call this when the user asks about their streak, consistency, or tracking habits.`,
		Execute: func(ctx context.Context, _ string) (string, error) {
			n, err := store.Streak(ctx, time.Now(), time.Local)
			if err != nil {
				return "", fmt.Errorf("get streak: %w", err)
			}
			switch n {
			case 0:
				return "No active streak — log a meal today to start one!", nil
			case 1:
				return "Current streak: 1 day. Keep it up!", nil
			default:
				return fmt.Sprintf("Current streak: %d days. Great consistency!", n), nil
			}
		},
	}
}

// getMealsTodayTool returns today's logged meals so the model can report calorie
// and macro totals without the user having to run a separate command.
func getMealsTodayTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "get_meals_today",
		Description: `Return all meals logged today with their calorie and macro totals.
Call this when the user asks about today's intake, remaining budget, or progress
toward a daily goal. No parameters needed.`,
		Execute: func(ctx context.Context, _ string) (string, error) {
			now := time.Now()
			meals, err := store.ListMealsForDay(ctx, now, time.Local)
			if err != nil {
				return "", fmt.Errorf("list meals: %w", err)
			}
			if len(meals) == 0 {
				return "No meals logged today.", nil
			}
			var totalKcal, totalP, totalC, totalF float64
			var sb strings.Builder
			for _, m := range meals {
				fmt.Fprintf(&sb, "- %s: %.0f kcal, %.1fg P, %.1fg C, %.1fg F\n",
					m.Title, m.Kcal, m.ProteinG, m.CarbsG, m.FatG)
				totalKcal += m.Kcal
				totalP += m.ProteinG
				totalC += m.CarbsG
				totalF += m.FatG
			}
			fmt.Fprintf(&sb, "Total: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
				totalKcal, totalP, totalC, totalF)
			return sb.String(), nil
		},
	}
}

// getMealsWeekTool returns this week's logged meals so the model can report
// weekly calorie and macro totals when the user asks.
func getMealsWeekTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "get_meals_week",
		Description: `Return all meals logged this week (Mon–today) with daily and weekly totals.
Call this when the user asks about their weekly intake, weekly progress,
or how they are tracking across the full week.`,
		Execute: func(ctx context.Context, _ string) (string, error) {
			now := time.Now()
			loc := time.Local
			from := weekStart(now, loc)
			to := from.AddDate(0, 0, 7)
			meals, err := store.ListMealsBetween(ctx, from, to)
			if err != nil {
				return "", fmt.Errorf("list meals: %w", err)
			}
			if len(meals) == 0 {
				return "No meals logged this week.", nil
			}

			// Ordered accumulation by day.
			order := []string{}
			groups := map[string][]db.Meal{}
			for _, m := range meals {
				label := m.EatenAt.In(loc).Format("Monday 2006-01-02")
				if _, seen := groups[label]; !seen {
					order = append(order, label)
				}
				groups[label] = append(groups[label], m)
			}

			var sb strings.Builder
			var weekKcal, weekP, weekC, weekF float64
			for _, label := range order {
				fmt.Fprintf(&sb, "%s:\n", label)
				var dayKcal, dayP, dayC, dayF float64
				for _, m := range groups[label] {
					fmt.Fprintf(&sb, "  - %s: %.0f kcal, %.1fg P, %.1fg C, %.1fg F\n",
						m.Title, m.Kcal, m.ProteinG, m.CarbsG, m.FatG)
					dayKcal += m.Kcal
					dayP += m.ProteinG
					dayC += m.CarbsG
					dayF += m.FatG
				}
				fmt.Fprintf(&sb, "  Day total: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat\n",
					dayKcal, dayP, dayC, dayF)
				weekKcal += dayKcal
				weekP += dayP
				weekC += dayC
				weekF += dayF
			}
			fmt.Fprintf(&sb, "Week total: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
				weekKcal, weekP, weekC, weekF)
			return sb.String(), nil
		},
	}
}

// setGoalsTool updates the user's daily nutrition targets from chat. Omitted
// fields are preserved; a field set to 0 clears its target.
func setGoalsTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "set_goals",
		Description: `Update the user's daily nutrition targets (kcal, protein, carbs, fat).
Call this when the user explicitly asks to set, update, or change a target
("set my calorie goal to 2000", "I want 150g protein daily"). Omit any
field the user did not mention — it will be preserved unchanged.
Setting a field to 0 clears that target.`,
		Parameters: map[string]*schema.ParameterInfo{
			"kcal": {
				Type: schema.Number,
				Desc: "Daily calorie target. Omit to keep existing value; 0 to clear.",
			},
			"protein_g": {
				Type: schema.Number,
				Desc: "Daily protein target in grams. Omit to keep; 0 to clear.",
			},
			"carbs_g": {
				Type: schema.Number,
				Desc: "Daily carbohydrate target in grams. Omit to keep; 0 to clear.",
			},
			"fat_g": {
				Type: schema.Number,
				Desc: "Daily fat target in grams. Omit to keep; 0 to clear.",
			},
		},
		Execute: func(ctx context.Context, args string) (string, error) {
			var p struct {
				Kcal     *float64 `json:"kcal"`
				ProteinG *float64 `json:"protein_g"`
				CarbsG   *float64 `json:"carbs_g"`
				FatG     *float64 `json:"fat_g"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}

			cur, err := store.GetGoals(ctx)
			if err != nil {
				return "", fmt.Errorf("get goals: %w", err)
			}

			// Merge: provided non-nil → override; 0 → clear; absent (nil) → keep current.
			merge := func(provided, current *float64) *float64 {
				if provided == nil {
					return current
				}
				if *provided == 0 {
					return nil
				}
				return provided
			}

			in := db.GoalInput{
				Kcal:     merge(p.Kcal, cur.Kcal),
				ProteinG: merge(p.ProteinG, cur.ProteinG),
				CarbsG:   merge(p.CarbsG, cur.CarbsG),
				FatG:     merge(p.FatG, cur.FatG),
			}

			updated, err := store.SetGoals(ctx, in)
			if err != nil {
				return "", fmt.Errorf("set goals: %w", err)
			}

			fmtField := func(label string, v *float64) string {
				if v == nil {
					return label + ": not set"
				}
				return fmt.Sprintf("%s: %.0f", label, *v)
			}
			return strings.Join([]string{
				"Goals updated:",
				fmtField("  kcal", updated.Kcal),
				fmtField("  protein_g", updated.ProteinG),
				fmtField("  carbs_g", updated.CarbsG),
				fmtField("  fat_g", updated.FatG),
			}, "\n"), nil
		},
	}
}

// deleteMealTool removes a logged meal by ID. The model calls this when the
// user asks to remove or undo a meal they logged by mistake.
func deleteMealTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "delete_meal",
		Description: `Delete a previously logged meal by its ID.
Call this when the user wants to remove a meal entry they logged by mistake
or wants to undo a recent log. Always confirm with the user before calling.`,
		Parameters: map[string]*schema.ParameterInfo{
			"meal_id": {
				Type:     schema.Integer,
				Desc:     "ID of the meal to delete (shown in the meals list).",
				Required: true,
			},
		},
		Execute: func(ctx context.Context, args string) (string, error) {
			var p struct {
				MealID int64 `json:"meal_id"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}
			meal, err := store.GetMeal(ctx, p.MealID)
			if err != nil {
				return "", fmt.Errorf("meal %d not found: %w", p.MealID, err)
			}
			if err := store.DeleteMeal(ctx, p.MealID); err != nil {
				return "", fmt.Errorf("delete meal: %w", err)
			}
			return fmt.Sprintf("deleted meal #%d: %s (%.0f kcal)", meal.ID, meal.Title, meal.Kcal), nil
		},
	}
}

// getGoalsTool fetches the user's daily nutrition targets so the model can
// compare actual intake against goals and give personalised advice.
func getGoalsTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "get_goals",
		Description: `Return the user's daily nutrition targets (kcal, protein, carbs, fat).
Call this when the user asks how close they are to their goals, whether
they have room for a meal, or when giving personalised dietary advice.`,
		Execute: func(ctx context.Context, _ string) (string, error) {
			g, err := store.GetGoals(ctx)
			if err != nil {
				return "", fmt.Errorf("get goals: %w", err)
			}
			format := func(label string, v *float64) string {
				if v == nil {
					return label + ": not set"
				}
				return fmt.Sprintf("%s: %.0f", label, *v)
			}
			return strings.Join([]string{
				"Daily targets:",
				format("  kcal", g.Kcal),
				format("  protein_g", g.ProteinG),
				format("  carbs_g", g.CarbsG),
				format("  fat_g", g.FatG),
			}, "\n"), nil
		},
	}
}

// logMealTool logs a meal to the user's diary. The model calls this whenever
// the user reports having eaten something.
func logMealTool(store db.Storer) ai.Tool {
	return ai.Tool{
		Name: "log_meal",
		Description: `Log a meal to the user's food diary.
Call this whenever the user reports having eaten something — even casually
("had a salad", "just finished lunch"). Estimate all nutritional fields you
can; leave unknown fields at 0.`,
		Parameters: map[string]*schema.ParameterInfo{
			"title": {
				Type:     schema.String,
				Desc:     "Short meal name (e.g. 'Chicken salad', 'Pasta bolognese').",
				Required: true,
			},
			"description": {
				Type: schema.String,
				Desc: "Longer description of the meal (ingredients, portion size, preparation).",
			},
			"items": {
				Type:     schema.Array,
				Desc:     "Individual food items.",
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
			},
			"kcal": {
				Type: schema.Number,
				Desc: "Estimated total kilocalories.",
			},
			"protein_g": {
				Type: schema.Number,
				Desc: "Estimated protein in grams.",
			},
			"carbs_g": {
				Type: schema.Number,
				Desc: "Estimated carbohydrates in grams.",
			},
			"fat_g": {
				Type: schema.Number,
				Desc: "Estimated fat in grams.",
			},
		},
		Execute: func(ctx context.Context, args string) (string, error) {
			var p struct {
				Title       string   `json:"title"`
				Description string   `json:"description"`
				Items       []string `json:"items"`
				Kcal        float64  `json:"kcal"`
				ProteinG    float64  `json:"protein_g"`
				CarbsG      float64  `json:"carbs_g"`
				FatG        float64  `json:"fat_g"`
			}
			if err := json.Unmarshal([]byte(args), &p); err != nil {
				return "", fmt.Errorf("parse args: %w", err)
			}
			meal, err := store.SaveMeal(ctx, db.MealInput{
				Title:       p.Title,
				Description: p.Description,
				Items:       p.Items,
				Kcal:        p.Kcal,
				ProteinG:    p.ProteinG,
				CarbsG:      p.CarbsG,
				FatG:        p.FatG,
			})
			if err != nil {
				return "", fmt.Errorf("save meal: %w", err)
			}
			return fmt.Sprintf(
				"meal logged (id=%d): %s — %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
				meal.ID, meal.Title, meal.Kcal, meal.ProteinG, meal.CarbsG, meal.FatG,
			), nil
		},
	}
}
