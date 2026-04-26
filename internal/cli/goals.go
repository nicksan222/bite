package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	cmd := &cobra.Command{
		Use:   "goals",
		Short: "View or set daily nutrition targets",
		Long: `View your current daily targets or update them.

Examples:
  bite goals                          # show current targets
  bite goals --kcal 2000              # set calorie target
  bite goals --protein 150 --kcal 2000 --carbs 200 --fat 60
  bite goals --kcal 0                 # clear calorie target (0 = unset)`,
		RunE: runGoals,
	}
	cmd.Flags().Float64P("kcal", "k", -1, "daily calorie target (0 = clear)")
	cmd.Flags().Float64P("protein", "p", -1, "daily protein target in grams (0 = clear)")
	cmd.Flags().Float64P("carbs", "c", -1, "daily carbohydrate target in grams (0 = clear)")
	cmd.Flags().Float64P("fat", "f", -1, "daily fat target in grams (0 = clear)")
	rootCmd.AddCommand(cmd)
}

func runGoals(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	return withStore(ctx, func(store db.Storer) error {
		return handleGoals(ctx, cmd.OutOrStdout(), cmd, store)
	})
}

// handleGoals either shows current goals or updates them, depending on which
// flags were passed. Accepts an explicit store so it can be unit-tested.
func handleGoals(ctx context.Context, out io.Writer, cmd *cobra.Command, store db.Storer) error {
	anySet := cmd.Flags().Changed("kcal") || cmd.Flags().Changed("protein") ||
		cmd.Flags().Changed("carbs") || cmd.Flags().Changed("fat")

	if !anySet {
		return showGoals(ctx, out, store)
	}

	cur, err := store.GetGoals(ctx)
	if err != nil {
		return fmt.Errorf("get goals: %w", err)
	}

	in := db.GoalInput{
		Kcal:     cur.Kcal,
		ProteinG: cur.ProteinG,
		CarbsG:   cur.CarbsG,
		FatG:     cur.FatG,
	}
	applyFlag(cmd, "kcal", &in.Kcal)
	applyFlag(cmd, "protein", &in.ProteinG)
	applyFlag(cmd, "carbs", &in.CarbsG)
	applyFlag(cmd, "fat", &in.FatG)

	updated, err := store.SetGoals(ctx, in)
	if err != nil {
		return fmt.Errorf("set goals: %w", err)
	}
	printAllGoals(out, updated)
	return nil
}

// showGoals fetches and prints the current goals to out.
func showGoals(ctx context.Context, out io.Writer, store db.Storer) error {
	g, err := store.GetGoals(ctx)
	if err != nil {
		return fmt.Errorf("get goals: %w", err)
	}
	printAllGoals(out, g)
	return nil
}

func printAllGoals(out io.Writer, g db.Goal) {
	fmt.Fprintln(out, "Daily targets:")
	printGoalField(out, "  kcal    ", g.Kcal)
	printGoalField(out, "  protein ", g.ProteinG)
	printGoalField(out, "  carbs   ", g.CarbsG)
	printGoalField(out, "  fat     ", g.FatG)
}

// applyFlag updates *field from a float64 flag: not Changed = unchanged,
// value <= 0 = clear (nil), value > 0 = set to that value.
func applyFlag(cmd *cobra.Command, name string, field **float64) {
	if !cmd.Flags().Changed(name) {
		return
	}
	v, _ := cmd.Flags().GetFloat64(name)
	if v <= 0 {
		*field = nil
	} else {
		*field = &v
	}
}

func printGoalField(out io.Writer, label string, v *float64) {
	if v == nil {
		fmt.Fprintf(out, "%s—\n", label)
	} else {
		fmt.Fprintf(out, "%s%.0f\n", label, *v)
	}
}
