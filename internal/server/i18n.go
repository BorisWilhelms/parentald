package server

import "net/http"

const defaultLang = "en"

var translations = map[string]map[string]string{
	"en": {
		// Nav
		"nav.users":    "Users",
		"nav.activity": "Activity",
		"nav.logout":   "Logout",

		// Login
		"login.username":    "Username",
		"login.password":    "Password",
		"login.submit":      "Sign in",
		"login.error":       "Invalid credentials",

		// Index
		"users.add":         "Add user",
		"users.placeholder": "Linux username",
		"users.submit":      "Add",

		// User list
		"user.locked":       "Locked until",
		"user.bonus":        "Bonus until",
		"user.delete":       "Delete",
		"user.delete.confirm": "Really delete user",
		"user.lock":         "Lock",
		"user.unlock":       "Unlock",
		"user.bonus.btn":    "Bonus",

		// Schedules
		"schedule.days":     "Days",
		"schedule.from":     "From",
		"schedule.to":       "To",
		"schedule.delete":   "Delete",
		"schedule.none":     "No schedules configured. User is unrestricted.",
		"schedule.add":      "Add schedule",
		"schedule.submit":   "Add",

		// Days
		"day.mon": "Mon",
		"day.tue": "Tue",
		"day.wed": "Wed",
		"day.thu": "Thu",
		"day.fri": "Fri",
		"day.sat": "Sat",
		"day.sun": "Sun",

		// Activity
		"activity.total":    "Total",
		"activity.other":    "Other",
		"activity.nodata":   "No activity data for this day.",

		// General
		"until.unlimited":   "unlimited",
	},
	"de": {
		// Nav
		"nav.users":    "Benutzer",
		"nav.activity": "Aktivität",
		"nav.logout":   "Abmelden",

		// Login
		"login.username":    "Benutzername",
		"login.password":    "Passwort",
		"login.submit":      "Anmelden",
		"login.error":       "Ungültige Anmeldedaten",

		// Index
		"users.add":         "Benutzer hinzufügen",
		"users.placeholder": "Linux-Benutzername",
		"users.submit":      "Hinzufügen",

		// User list
		"user.locked":       "Gesperrt bis",
		"user.bonus":        "Bonus bis",
		"user.delete":       "Löschen",
		"user.delete.confirm": "Benutzer wirklich löschen",
		"user.lock":         "Sperren",
		"user.unlock":       "Entsperren",
		"user.bonus.btn":    "Bonus",

		// Schedules
		"schedule.days":     "Tage",
		"schedule.from":     "Von",
		"schedule.to":       "Bis",
		"schedule.delete":   "Löschen",
		"schedule.none":     "Keine Zeitfenster konfiguriert. Benutzer ist nicht eingeschränkt.",
		"schedule.add":      "Zeitfenster hinzufügen",
		"schedule.submit":   "Hinzufügen",

		// Days
		"day.mon": "Mo",
		"day.tue": "Di",
		"day.wed": "Mi",
		"day.thu": "Do",
		"day.fri": "Fr",
		"day.sat": "Sa",
		"day.sun": "So",

		// Activity
		"activity.total":    "Gesamt",
		"activity.other":    "Sonstiges",
		"activity.nodata":   "Keine Aktivitätsdaten für diesen Tag.",

		// General
		"until.unlimited":   "unbegrenzt",
	},
}

// t translates a key for the given language.
func t(lang, key string) string {
	if m, ok := translations[lang]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	// Fall back to English
	if m, ok := translations["en"]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

// getLang reads the language preference from the cookie.
func getLang(r *http.Request) string {
	c, err := r.Cookie("lang")
	if err != nil || (c.Value != "en" && c.Value != "de") {
		return defaultLang
	}
	return c.Value
}
