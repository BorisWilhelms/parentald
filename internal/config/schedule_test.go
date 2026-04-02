package config

import (
	"testing"
	"time"
)

func makeTime(weekday time.Weekday, hour, minute int) time.Time {
	// Find a date that falls on the given weekday.
	// Start from a known Monday (2024-01-01).
	base := time.Date(2024, 1, 1, hour, minute, 0, 0, time.Local)
	for base.Weekday() != weekday {
		base = base.AddDate(0, 0, 1)
	}
	return base
}

func TestIsAllowed_UserNotInConfig(t *testing.T) {
	cfg := Config{Users: map[string]User{}}
	if !IsAllowed("unknown", cfg, time.Now()) {
		t.Error("user not in config should be allowed")
	}
}

func TestIsAllowed_WithinSchedule(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules: []Schedule{
					{Days: []string{"mon", "tue", "wed", "thu", "fri"}, From: "14:00", To: "18:00"},
				},
			},
		},
	}

	// Monday 15:00 — should be allowed
	now := makeTime(time.Monday, 15, 0)
	if !IsAllowed("kind1", cfg, now) {
		t.Error("should be allowed at Mon 15:00")
	}
}

func TestIsAllowed_OutsideSchedule(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules: []Schedule{
					{Days: []string{"mon"}, From: "14:00", To: "18:00"},
				},
			},
		},
	}

	// Monday 12:00 — should be denied
	now := makeTime(time.Monday, 12, 0)
	if IsAllowed("kind1", cfg, now) {
		t.Error("should be denied at Mon 12:00")
	}
}

func TestIsAllowed_WrongDay(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules: []Schedule{
					{Days: []string{"mon"}, From: "14:00", To: "18:00"},
				},
			},
		},
	}

	// Tuesday 15:00 — should be denied (schedule only for Monday)
	now := makeTime(time.Tuesday, 15, 0)
	if IsAllowed("kind1", cfg, now) {
		t.Error("should be denied on Tuesday")
	}
}

func TestIsAllowed_MultipleSchedules(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules: []Schedule{
					{Days: []string{"mon", "tue", "wed", "thu", "fri"}, From: "14:00", To: "18:00"},
					{Days: []string{"sat", "sun"}, From: "10:00", To: "20:00"},
				},
			},
		},
	}

	// Saturday 11:00 — should be allowed by second schedule
	now := makeTime(time.Saturday, 11, 0)
	if !IsAllowed("kind1", cfg, now) {
		t.Error("should be allowed at Sat 11:00")
	}

	// Saturday 21:00 — should be denied
	now = makeTime(time.Saturday, 21, 0)
	if IsAllowed("kind1", cfg, now) {
		t.Error("should be denied at Sat 21:00")
	}
}

func TestIsAllowed_BoundaryExact(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules: []Schedule{
					{Days: []string{"mon"}, From: "14:00", To: "18:00"},
				},
			},
		},
	}

	// Exactly at From — allowed
	now := makeTime(time.Monday, 14, 0)
	if !IsAllowed("kind1", cfg, now) {
		t.Error("should be allowed at exactly From time")
	}

	// Exactly at To — denied (exclusive end)
	now = makeTime(time.Monday, 18, 0)
	if IsAllowed("kind1", cfg, now) {
		t.Error("should be denied at exactly To time")
	}
}

func TestIsAllowed_CaseInsensitiveDays(t *testing.T) {
	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules: []Schedule{
					{Days: []string{"Mon", "TUE"}, From: "10:00", To: "12:00"},
				},
			},
		},
	}

	now := makeTime(time.Monday, 11, 0)
	if !IsAllowed("kind1", cfg, now) {
		t.Error("day matching should be case insensitive")
	}
}
