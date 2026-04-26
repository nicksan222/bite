package tools

import (
	"fmt"
	"strings"

	"github.com/nicksan222/bite/internal/db"
)

// formatGoals renders a Goal as a multi-line block under header. nil fields
// render as "not set" so users see explicitly which targets are unconfigured
// rather than a misleading "0". Used by both get_goals (after a fresh read)
// and set_goals (after an update) so the two views stay in sync.
func formatGoals(header string, g db.Goal) string {
	field := func(label string, v *float64) string {
		if v == nil {
			return label + ": not set"
		}
		return fmt.Sprintf("%s: %.0f", label, *v)
	}
	return strings.Join([]string{
		header,
		field("  kcal", g.Kcal),
		field("  protein_g", g.ProteinG),
		field("  carbs_g", g.CarbsG),
		field("  fat_g", g.FatG),
	}, "\n")
}
