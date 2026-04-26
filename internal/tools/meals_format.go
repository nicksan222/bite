package tools

import (
	"fmt"
	"strings"

	"github.com/nicksan222/bite/internal/db"
)

// totals accumulates calorie and macro sums across a meal slice. Kept as a
// concrete struct (rather than four float64s) so callers can pass it around
// and have one place to format it.
type totals struct{ Kcal, ProteinG, CarbsG, FatG float64 }

func (t *totals) add(m db.Meal) {
	t.Kcal += m.Kcal
	t.ProteinG += m.ProteinG
	t.CarbsG += m.CarbsG
	t.FatG += m.FatG
}

// addTotals folds another totals into the receiver. Used by meals_week to
// accumulate the week sum from each day's totals without four boilerplate
// field-by-field additions at the call site.
func (t *totals) addTotals(o totals) {
	t.Kcal += o.Kcal
	t.ProteinG += o.ProteinG
	t.CarbsG += o.CarbsG
	t.FatG += o.FatG
}

// writeMealLine writes one meal as a bullet line: "- title: K kcal, Pg P, Cg C, Fg F".
// indent is prepended to the bullet so the helper works for both flat lists and
// daily groupings.
func writeMealLine(sb *strings.Builder, indent string, m db.Meal) {
	fmt.Fprintf(sb, "%s- %s: %.0f kcal, %.1fg P, %.1fg C, %.1fg F\n",
		indent, m.Title, m.Kcal, m.ProteinG, m.CarbsG, m.FatG)
}

// formatMacrosFull renders the long-form macro tail "X kcal, Yg protein,
// Zg carbs, Wg fat" used by both writeTotalsLine and formatMealSaved. One
// formatter keeps the precision (kcal int, macros 1 decimal) and field
// order locked across every surface that confirms a meal or sums a day.
func formatMacrosFull(kcal, protein, carbs, fat float64) string {
	return fmt.Sprintf("%.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
		kcal, protein, carbs, fat)
}

// macroCells renders the four macro columns for a tabular meal listing as
// integer-rounded strings. Used by meals_recent for both row data and the
// totals footer; centralised so the column precision/formatting stays
// consistent across every cell of the same table.
func macroCells(kcal, protein, carbs, fat float64) []string {
	return []string{
		fmt.Sprintf("%.0f", kcal),
		fmt.Sprintf("%.0f", protein),
		fmt.Sprintf("%.0f", carbs),
		fmt.Sprintf("%.0f", fat),
	}
}

// writeTotalsLine writes "Total: K kcal, Pg protein, Cg carbs, Fg fat" with
// the supplied label (e.g. "Total" or "  Day total").
func writeTotalsLine(sb *strings.Builder, label string, t totals) {
	fmt.Fprintf(sb, "%s: %s", label, formatMacrosFull(t.Kcal, t.ProteinG, t.CarbsG, t.FatG))
}

// formatMealSaved is the canonical "logged a meal" confirmation line, shared
// between log_meal (text-only) and log_meal_from_media (media + text). One
// formatter keeps the wording, precision, and field order consistent across
// both surfaces — matters because the chat history shows assistant turns,
// and the model uses prior confirmations as context for follow-ups.
func formatMealSaved(m db.Meal) string {
	return fmt.Sprintf("logged #%d %s — %s",
		m.ID, m.Title, formatMacrosFull(m.Kcal, m.ProteinG, m.CarbsG, m.FatG))
}

// formatMealList produces a flat bullet list of meals followed by a single
// totals line. Used by meals_today and meals_on.
func formatMealList(meals []db.Meal) string {
	var sb strings.Builder
	var tot totals
	for _, m := range meals {
		writeMealLine(&sb, "", m)
		tot.add(m)
	}
	writeTotalsLine(&sb, "Total", tot)
	return sb.String()
}
