package config

import (
	"fmt"
	"strings"
	"time"
)

var weekdayMap = map[time.Weekday]string{
	time.Monday:    "mon",
	time.Tuesday:   "tue",
	time.Wednesday: "wed",
	time.Thursday:  "thu",
	time.Friday:    "fri",
	time.Saturday:  "sat",
	time.Sunday:    "sun",
}

// IsAllowed checks whether a user is allowed to be logged in at the given time.
// Users not present in the config are always allowed (only configured users are restricted).
//
// Priority: LockedUntil > BonusUntil > Schedule
func IsAllowed(username string, cfg Config, now time.Time) bool {
	user, ok := cfg.Users[username]
	if !ok {
		return true
	}

	// 1. Instant lock beats everything
	if user.LockedUntil != nil && now.Before(*user.LockedUntil) {
		return false
	}

	// 2. Bonus time grants access
	if user.BonusUntil != nil && now.Before(*user.BonusUntil) {
		return true
	}

	// 3. Check schedules
	return IsInSchedule(user.Schedules, now)
}

// isInSchedule checks if the given time falls within any of the schedules.
func IsInSchedule(schedules []Schedule, now time.Time) bool {
	day := strings.ToLower(weekdayMap[now.Weekday()])
	nowMinutes := now.Hour()*60 + now.Minute()

	for _, s := range schedules {
		if !containsDay(s.Days, day) {
			continue
		}

		from, err := parseTime(s.From)
		if err != nil {
			continue
		}
		to, err := parseTime(s.To)
		if err != nil {
			continue
		}

		if nowMinutes >= from && nowMinutes < to {
			return true
		}
	}

	return false
}

// NextScheduleStart returns the start time of the next schedule window after now.
// Returns nil if there are no schedules.
func NextScheduleStart(user User, now time.Time) *time.Time {
	if len(user.Schedules) == 0 {
		return nil
	}

	nowMinutes := now.Hour()*60 + now.Minute()
	var best *time.Time

	// Check today and the next 7 days
	for dayOffset := 0; dayOffset < 8; dayOffset++ {
		candidate := now.AddDate(0, 0, dayOffset)
		day := strings.ToLower(weekdayMap[candidate.Weekday()])

		for _, s := range user.Schedules {
			if !containsDay(s.Days, day) {
				continue
			}

			from, err := parseTime(s.From)
			if err != nil {
				continue
			}

			// On the first day (today), only consider schedules that start after now
			if dayOffset == 0 && from <= nowMinutes {
				continue
			}

			t := time.Date(candidate.Year(), candidate.Month(), candidate.Day(),
				from/60, from%60, 0, 0, now.Location())

			if best == nil || t.Before(*best) {
				best = &t
			}
		}
	}

	return best
}

// ScheduleEndTime returns the end time of the currently active schedule window.
// Returns nil if the user is not in any schedule right now.
func ScheduleEndTime(user User, now time.Time) *time.Time {
	day := strings.ToLower(weekdayMap[now.Weekday()])
	nowMinutes := now.Hour()*60 + now.Minute()

	for _, s := range user.Schedules {
		if !containsDay(s.Days, day) {
			continue
		}

		from, err := parseTime(s.From)
		if err != nil {
			continue
		}
		to, err := parseTime(s.To)
		if err != nil {
			continue
		}

		if nowMinutes >= from && nowMinutes < to {
			t := time.Date(now.Year(), now.Month(), now.Day(),
				to/60, to%60, 0, 0, now.Location())
			return &t
		}
	}

	return nil
}

func containsDay(days []string, day string) bool {
	for _, d := range days {
		if strings.ToLower(d) == day {
			return true
		}
	}
	return false
}

// parseTime parses "HH:MM" into minutes since midnight.
func parseTime(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q: %w", s, err)
	}
	return t.Hour()*60 + t.Minute(), nil
}
