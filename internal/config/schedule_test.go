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

func timePtr(t time.Time) *time.Time {
	return &t
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

	now := makeTime(time.Saturday, 11, 0)
	if !IsAllowed("kind1", cfg, now) {
		t.Error("should be allowed at Sat 11:00")
	}

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

// --- Instant Lock tests ---

func TestIsAllowed_LockedBeatsSchedule(t *testing.T) {
	now := makeTime(time.Monday, 15, 0)
	lockUntil := now.Add(2 * time.Hour)

	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules:   []Schedule{{Days: []string{"mon"}, From: "14:00", To: "18:00"}},
				LockedUntil: &lockUntil,
			},
		},
	}

	if IsAllowed("kind1", cfg, now) {
		t.Error("locked user should be denied even during schedule")
	}
}

func TestIsAllowed_LockedBeatsBonus(t *testing.T) {
	now := makeTime(time.Monday, 15, 0)
	lockUntil := now.Add(2 * time.Hour)
	bonusUntil := now.Add(1 * time.Hour)

	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules:   []Schedule{{Days: []string{"mon"}, From: "14:00", To: "18:00"}},
				LockedUntil: &lockUntil,
				BonusUntil:  &bonusUntil,
			},
		},
	}

	if IsAllowed("kind1", cfg, now) {
		t.Error("lock should beat bonus")
	}
}

func TestIsAllowed_ExpiredLockIgnored(t *testing.T) {
	now := makeTime(time.Monday, 15, 0)
	lockUntil := now.Add(-1 * time.Hour) // expired

	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules:   []Schedule{{Days: []string{"mon"}, From: "14:00", To: "18:00"}},
				LockedUntil: &lockUntil,
			},
		},
	}

	if !IsAllowed("kind1", cfg, now) {
		t.Error("expired lock should be ignored, schedule should allow")
	}
}

// --- Bonus time tests ---

func TestIsAllowed_BonusOutsideSchedule(t *testing.T) {
	now := makeTime(time.Monday, 19, 0) // outside schedule
	bonusUntil := now.Add(30 * time.Minute)

	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules:  []Schedule{{Days: []string{"mon"}, From: "14:00", To: "18:00"}},
				BonusUntil: &bonusUntil,
			},
		},
	}

	if !IsAllowed("kind1", cfg, now) {
		t.Error("bonus should grant access outside schedule")
	}
}

func TestIsAllowed_ExpiredBonusIgnored(t *testing.T) {
	now := makeTime(time.Monday, 19, 0)
	bonusUntil := now.Add(-10 * time.Minute) // expired

	cfg := Config{
		Users: map[string]User{
			"kind1": {
				Schedules:  []Schedule{{Days: []string{"mon"}, From: "14:00", To: "18:00"}},
				BonusUntil: &bonusUntil,
			},
		},
	}

	if IsAllowed("kind1", cfg, now) {
		t.Error("expired bonus should be ignored")
	}
}

// --- NextScheduleStart tests ---

func TestNextScheduleStart_LaterToday(t *testing.T) {
	now := makeTime(time.Monday, 12, 0)
	user := User{
		Schedules: []Schedule{
			{Days: []string{"mon"}, From: "14:00", To: "18:00"},
		},
	}

	next := NextScheduleStart(user, now)
	if next == nil {
		t.Fatal("expected a next schedule start")
	}
	if next.Hour() != 14 || next.Minute() != 0 {
		t.Errorf("expected 14:00, got %02d:%02d", next.Hour(), next.Minute())
	}
}

func TestNextScheduleStart_Tomorrow(t *testing.T) {
	now := makeTime(time.Monday, 19, 0) // after today's schedule
	user := User{
		Schedules: []Schedule{
			{Days: []string{"mon", "tue"}, From: "14:00", To: "18:00"},
		},
	}

	next := NextScheduleStart(user, now)
	if next == nil {
		t.Fatal("expected a next schedule start")
	}
	if next.Weekday() != time.Tuesday {
		t.Errorf("expected Tuesday, got %s", next.Weekday())
	}
}

func TestNextScheduleStart_NoSchedules(t *testing.T) {
	now := makeTime(time.Monday, 12, 0)
	user := User{}

	next := NextScheduleStart(user, now)
	if next != nil {
		t.Error("expected nil for no schedules")
	}
}

func TestNextScheduleStart_CurrentlyInSchedule(t *testing.T) {
	now := makeTime(time.Monday, 15, 0) // inside 14:00-18:00
	user := User{
		Schedules: []Schedule{
			{Days: []string{"mon", "tue"}, From: "14:00", To: "18:00"},
		},
	}

	next := NextScheduleStart(user, now)
	if next == nil {
		t.Fatal("expected a next schedule start")
	}
	// Should skip current schedule (14:00 <= 15:00), return tomorrow's
	if next.Weekday() != time.Tuesday {
		t.Errorf("expected Tuesday, got %s", next.Weekday())
	}
}

// --- ScheduleEndTime tests ---

func TestScheduleEndTime_InSchedule(t *testing.T) {
	now := makeTime(time.Monday, 15, 0)
	user := User{
		Schedules: []Schedule{
			{Days: []string{"mon"}, From: "14:00", To: "18:00"},
		},
	}

	end := ScheduleEndTime(user, now)
	if end == nil {
		t.Fatal("expected schedule end time")
	}
	if end.Hour() != 18 || end.Minute() != 0 {
		t.Errorf("expected 18:00, got %02d:%02d", end.Hour(), end.Minute())
	}
}

func TestScheduleEndTime_OutsideSchedule(t *testing.T) {
	now := makeTime(time.Monday, 19, 0)
	user := User{
		Schedules: []Schedule{
			{Days: []string{"mon"}, From: "14:00", To: "18:00"},
		},
	}

	end := ScheduleEndTime(user, now)
	if end != nil {
		t.Error("expected nil outside schedule")
	}
}
