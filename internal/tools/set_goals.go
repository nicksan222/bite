package tools

import (
	"context"
	"fmt"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	Register(Tool{
		Name:    "set_goals",
		Summary: "Set or update daily nutrition targets.",
		Description: `Update the user's daily nutrition targets (kcal, protein, carbs, fat).
Omit any field the user did not mention — it will be preserved unchanged.
Setting a field to 0 clears that target.`,
		Prompt: `Call set_goals when the user explicitly asks to set, update, or change a target
("set my calorie goal to 2000", "I want 150g protein daily"). Omit any field
they did not mention; it will be preserved.`,
		Examples: []Example{
			{Cmd: "bite set_goals --kcal=2000 --protein-g=150", Desc: "set daily targets"},
		},
		Params: []Param{
			{Name: "kcal", Type: ParamFloat,
				Desc: "Daily calorie target. Omit to keep existing value; 0 to clear."},
			{Name: "protein_g", Type: ParamFloat,
				Desc: "Daily protein target in grams. Omit to keep; 0 to clear."},
			{Name: "carbs_g", Type: ParamFloat,
				Desc: "Daily carbohydrate target in grams. Omit to keep; 0 to clear."},
			{Name: "fat_g", Type: ParamFloat,
				Desc: "Daily fat target in grams. Omit to keep; 0 to clear."},
		},
		Run: runSetGoals,
	})
}

func runSetGoals(ctx context.Context, deps Deps, args Args) (Result, error) {
	cur, err := deps.Store.GetGoals(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("get goals: %w", err)
	}

	in := db.GoalInput{
		Kcal:     mergeGoal(args, "kcal", cur.Kcal),
		ProteinG: mergeGoal(args, "protein_g", cur.ProteinG),
		CarbsG:   mergeGoal(args, "carbs_g", cur.CarbsG),
		FatG:     mergeGoal(args, "fat_g", cur.FatG),
	}

	updated, err := deps.Store.SetGoals(ctx, in)
	if err != nil {
		return Result{}, fmt.Errorf("set goals: %w", err)
	}
	return Result{Text: formatGoals("Goals updated:", updated)}, nil
}

// mergeGoal applies absent-vs-zero semantics: absent → keep current,
// supplied 0 → clear (nil), supplied non-zero → use new value.
func mergeGoal(args Args, name string, current *float64) *float64 {
	if !args.Has(name) {
		return current
	}
	v := args.Float(name)
	if v == 0 {
		return nil
	}
	return &v
}
