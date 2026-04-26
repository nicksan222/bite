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

// writeTotalsLine writes "Total: K kcal, Pg protein, Cg carbs, Fg fat" with
// the supplied label (e.g. "Total" or "  Day total").
func writeTotalsLine(sb *strings.Builder, label string, t totals) {
	fmt.Fprintf(sb, "%s: %.0f kcal, %.1fg protein, %.1fg carbs, %.1fg fat",
		label, t.Kcal, t.ProteinG, t.CarbsG, t.FatG)
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
