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
func IsAllowed(username string, cfg Config, now time.Time) bool {
	user, ok := cfg.Users[username]
	if !ok {
		return true
	}

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
			return true
		}
	}

	return false
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
