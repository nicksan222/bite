package db

import (
	"context"
	"time"
)

// Streak returns the number of consecutive calendar days (ending today or
// yesterday) on which at least one meal was logged.
//
// The look-back window is capped at 366 days for performance: a streak can
// never be longer than a year.
func (s *Store) Streak(ctx context.Context, now time.Time, loc *time.Location) (int, error) {
	if loc == nil {
		loc = time.Local
	}
	// Fetch the last 366 days in one query.
	end := startOfDay(now, loc).Add(24 * time.Hour)
	start := end.AddDate(0, 0, -366)
	meals, err := s.ListMealsBetween(ctx, start, end)
	if err != nil {
		return 0, err
	}
	return computeStreak(meals, now, loc), nil
}

// computeStreak is the pure, testable streak logic.
// It counts consecutive days (Mon→…→today) that have at least one meal,
// working backwards from today (or yesterday if today has no meals yet).
func computeStreak(meals []Meal, now time.Time, loc *time.Location) int {
	// Build a set of dates that have at least one meal.
	type dateKey struct {
		y int
		m time.Month
		d int
	}
	logged := make(map[dateKey]bool, len(meals))
	for _, m := range meals {
		t := m.EatenAt.In(loc)
		logged[dateKey{t.Year(), t.Month(), t.Day()}] = true
	}

	today := startOfDay(now, loc)
	todayKey := dateKey{today.Year(), today.Month(), today.Day()}

	// If today has no meals, don't break the streak — start counting from
	// yesterday so an early-morning session doesn't reset a long streak.
	start := today
	if !logged[todayKey] {
		start = today.AddDate(0, 0, -1)
	}

	count := 0
	for d := start; ; d = d.AddDate(0, 0, -1) {
		k := dateKey{d.Year(), d.Month(), d.Day()}
		if !logged[k] {
			break
		}
		count++
	}
	return count
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc)
}
