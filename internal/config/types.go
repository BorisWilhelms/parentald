package config

import "time"

// Config is the top-level configuration structure.
type Config struct {
	Users map[string]User `json:"users"`
}

// User holds the screen time schedules for a single user.
type User struct {
	Schedules   []Schedule `json:"schedules"`
	LockedUntil *time.Time `json:"lockedUntil,omitempty"`
	BonusUntil  *time.Time `json:"bonusUntil,omitempty"`
}

// Schedule defines an allowed time window on specific days.
type Schedule struct {
	Days []string `json:"days"` // "mon","tue","wed","thu","fri","sat","sun"
	From string   `json:"from"` // "HH:MM" (24h)
	To   string   `json:"to"`   // "HH:MM" (24h), must be > From
}
