package cli

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/nicksan222/bite/internal/db"
)

func init() {
	cmd := &cobra.Command{
		Use:   "meals",
		Short: "Show or manage logged meals",
		Long: `Show meals logged today (default) or for a given date range.

Examples:
  bite meals                    # today's meals
  bite meals --date 2026-04-20  # meals logged on a specific date
  bite meals --recent 10        # last 10 meals regardless of date
  bite meals --week             # this week's meals with daily totals
  bite meals delete 42          # delete meal with ID 42`,
		RunE: runMeals,
	}
	cmd.Flags().Int64P("recent", "n", 0, "show the N most recent meals across all dates")
	cmd.Flags().StringP("date", "d", "", "show meals for a specific date (YYYY-MM-DD)")
	cmd.Flags().BoolP("week", "w", false, "show this week's meals grouped by day")

	cmd.AddCommand(&cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a meal by ID",
		Args:  cobra.ExactArgs(1),
		RunE:  runDeleteMeal,
	})

	rootCmd.AddCommand(cmd)
}

func runDeleteMeal(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	id, err := parseID(args[0])
	if err != nil {
		return err
	}
	return withStore(ctx, func(store db.Storer) error {
		return deleteMeal(ctx, cmd.OutOrStdout(), store, id)
	})
}

func deleteMeal(ctx context.Context, out io.Writer, store db.Storer, id int64) error {
	meal, err := store.GetMeal(ctx, id)
	if err != nil {
		return fmt.Errorf("meal %d not found: %w", id, err)
	}
	if err := store.DeleteMeal(ctx, id); err != nil {
		return fmt.Errorf("delete meal: %w", err)
	}
	fmt.Fprintf(out, "deleted meal #%d: %s\n", meal.ID, meal.Title)
	return nil
}

func runMeals(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	return withStore(ctx, func(store db.Storer) error {
		if week, _ := cmd.Flags().GetBool("week"); week {
			return showMealsWeek(ctx, cmd.OutOrStdout(), store, time.Now(), time.Local)
		}

		n, _ := cmd.Flags().GetInt64("recent")

		now := time.Now()
		if d, _ := cmd.Flags().GetString("date"); d != "" {
			parsed, err := time.ParseInLocation("2006-01-02", d, time.Local)
			if err != nil {
				return fmt.Errorf("invalid date %q — use YYYY-MM-DD: %w", d, err)
			}
			now = parsed
		}

		return showMeals(ctx, cmd.OutOrStdout(), store, n, now, time.Local)
	})
}

// weekStart returns the Monday of the week containing t in loc.
func weekStart(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	y, m, d := t.Date()
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7 // Sunday → 7, so Monday is offset -6
	}
	return time.Date(y, m, d-wd+1, 0, 0, 0, 0, loc)
}

// showMealsWeek prints meals for the current week grouped by calendar day.
func showMealsWeek(ctx context.Context, out io.Writer, store db.Storer, now time.Time, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}
	from := weekStart(now, loc)
	to := from.AddDate(0, 0, 7)

	meals, err := store.ListMealsBetween(ctx, from, to)
	if err != nil {
		return err
	}

	if len(meals) == 0 {
		fmt.Fprintln(out, "no meals logged this week")
		return nil
	}

	// Group by calendar day.
	type dayKey struct {
		y int
		m time.Month
		d int
	}
	order := []dayKey{}
	groups := map[dayKey][]db.Meal{}
	for _, meal := range meals {
		t := meal.EatenAt.In(loc)
		k := dayKey{t.Year(), t.Month(), t.Day()}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], meal)
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	var weekKcal, weekP, weekC, weekF float64
	for _, k := range order {
		day := time.Date(k.y, k.m, k.d, 0, 0, 0, 0, loc)
		fmt.Fprintf(tw, "\n%s\n", day.Format("Monday 2006-01-02"))
		fmt.Fprintln(tw, "  ID\tTIME\tTITLE\tKCAL\tP\tC\tF")
		var dayKcal, dayP, dayC, dayF float64
		for _, meal := range groups[k] {
			fmt.Fprintf(tw, "  %d\t%s\t%s\t%.0f\t%.0f\t%.0f\t%.0f\n",
				meal.ID, meal.EatenAt.In(loc).Format("15:04"), meal.Title,
				meal.Kcal, meal.ProteinG, meal.CarbsG, meal.FatG)
			dayKcal += meal.Kcal
			dayP += meal.ProteinG
			dayC += meal.CarbsG
			dayF += meal.FatG
		}
		fmt.Fprintf(tw, "  total\t\t\t%.0f\t%.0f\t%.0f\t%.0f\n", dayKcal, dayP, dayC, dayF)
		weekKcal += dayKcal
		weekP += dayP
		weekC += dayC
		weekF += dayF
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "\nweek total: %.0f kcal · P %.0fg · C %.0fg · F %.0fg\n",
		weekKcal, weekP, weekC, weekF)
	return nil
}

// showMeals prints meal rows. If n > 0 it uses ListRecentMeals; otherwise
// it lists today's meals using now as the reference point.
func showMeals(ctx context.Context, out io.Writer, store db.Storer, recent int64, now time.Time, loc *time.Location) error {
	if loc == nil {
		loc = time.Local
	}
	var meals []db.Meal
	var err error

	if recent > 0 {
		meals, err = store.ListRecentMeals(ctx, recent)
	} else {
		meals, err = store.ListMealsForDay(ctx, now, loc)
	}
	if err != nil {
		return err
	}

	if len(meals) == 0 {
		fmt.Fprintln(out, "no meals logged yet")
		return nil
	}

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tTIME\tTITLE\tKCAL\tP\tC\tF")
	var totalKcal, totalP, totalC, totalF float64
	for _, m := range meals {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%.0f\t%.0f\t%.0f\t%.0f\n",
			m.ID, m.EatenAt.In(loc).Format("15:04"), m.Title,
			m.Kcal, m.ProteinG, m.CarbsG, m.FatG)
		totalKcal += m.Kcal
		totalP += m.ProteinG
		totalC += m.CarbsG
		totalF += m.FatG
	}
	if err := tw.Flush(); err != nil {
		return err
	}

	fmt.Fprintf(out, "\ntotal: %.0f kcal · P %.0fg · C %.0fg · F %.0fg\n",
		totalKcal, totalP, totalC, totalF)
	return nil
}
