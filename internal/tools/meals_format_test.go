package tools

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/nicksan222/bite/internal/db"
)

func TestFormatMealList_summarisesAndTotals(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	meals := []db.Meal{
		{Title: "Eggs", Kcal: 200, ProteinG: 18, CarbsG: 1, FatG: 14, EatenAt: now},
		{Title: "Toast", Kcal: 150, ProteinG: 4, CarbsG: 30, FatG: 2, EatenAt: now},
	}
	out := formatMealList(meals)
	assert.Contains(t, out, "- Eggs:")
	assert.Contains(t, out, "- Toast:")
	assert.Contains(t, out, "Total: 350 kcal", "totals must accumulate kcal")
	assert.Contains(t, out, "22.0g protein", "totals must accumulate protein")
}

func TestTotals_addAccumulates(t *testing.T) {
	var tot totals
	tot.add(db.Meal{Kcal: 100, ProteinG: 10, CarbsG: 20, FatG: 5})
	tot.add(db.Meal{Kcal: 50, ProteinG: 5, CarbsG: 10, FatG: 2})
	assert.Equal(t, 150.0, tot.Kcal)
	assert.Equal(t, 15.0, tot.ProteinG)
	assert.Equal(t, 30.0, tot.CarbsG)
	assert.Equal(t, 7.0, tot.FatG)
}
