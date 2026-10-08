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
		"login.username": "Username",
		"login.password": "Password",
		"login.submit":   "Sign in",
		"login.error":    "Invalid credentials",

		// Index
		"users.add":         "Add user",
		"users.placeholder": "Linux username",
		"users.submit":      "Add",

		// User list
		"user.locked":         "Locked until",
		"user.bonus":          "Bonus until",
		"user.delete":         "Delete",
		"user.delete.confirm": "Really delete user",
		"user.lock":           "Lock",
		"user.unlock":         "Unlock",
		"user.bonus.btn":      "Bonus",

		// Schedules
		"schedule.days":   "Days",
		"schedule.from":   "From",
		"schedule.to":     "To",
		"schedule.delete": "Delete",
		"schedule.none":   "No schedules configured. User is unrestricted.",
		"schedule.add":    "Add schedule",
		"schedule.submit": "Add",

		// Days
		"day.mon": "Mon",
		"day.tue": "Tue",
		"day.wed": "Wed",
		"day.thu": "Thu",
		"day.fri": "Fri",
		"day.sat": "Sat",
		"day.sun": "Sun",

		// Activity
		"activity.screentime":     "Screen time",
		"activity.screentime.tip": "Time the user was actively logged in (not idle or locked)",
		"activity.cattotal.tip":   "Sum of all app runtimes in this category (apps can run in parallel)",
		"activity.other":          "Other",
		"activity.nodata":         "No activity data for this day.",

		// Dashboard
		"nav.dashboard":        "Dashboard",
		"dashboard.user":       "User",
		"dashboard.from":       "From",
		"dashboard.to":         "To",
		"dashboard.update":     "Update",
		"dashboard.screentime": "Screen Time per Day",
		"dashboard.apptime":    "App Usage per Day",
		"dashboard.filterApp":  "Filter app",
		"dashboard.allApps":    "All apps",

		// Redesign
		"state.locked":             "Locked",
		"state.bonus":              "Bonus time",
		"state.free":               "Unrestricted",
		"state.allowed":            "Allowed now",
		"state.blocked":            "Outside allowed hours",
		"status.online":            "Online",
		"status.idle":              "Idle",
		"status.offline":           "Offline",
		"status.never":             "Not seen yet",
		"user.today":               "Today",
		"user.until":               "until",
		"user.bonus.add":           "Add bonus time",
		"user.bonus.custom":        "Min.",
		"user.schedule":            "Schedule",
		"user.noactivity":          "No activity today",
		"users.empty":              "No users yet. Add a Linux user to get started.",
		"schedule.preset.weekdays": "Mon–Fri",
		"schedule.preset.weekend":  "Weekend",
		"schedule.preset.all":      "Every day",
		"date.today":               "Today",
		"date.yesterday":           "Yesterday",
		"weekday.mon":              "Monday",
		"weekday.tue":              "Tuesday",
		"weekday.wed":              "Wednesday",
		"weekday.thu":              "Thursday",
		"weekday.fri":              "Friday",
		"weekday.sat":              "Saturday",
		"weekday.sun":              "Sunday",
		"activity.apps":            "Apps",
		"activity.categories":      "Categories",
		"cat.Game":                 "Games",
		"cat.Network":              "Internet",
		"cat.Education":            "Education",
		"cat.Office":               "Office",
		"cat.AudioVideo":           "Media",
		"cat.Graphics":             "Graphics",
		"cat.Development":          "Development",
		"cat.Utility":              "Utilities",
		"cat.System":               "System",
		"cat.Settings":             "Settings",
		"dashboard.range.7":        "7 days",
		"dashboard.range.14":       "14 days",
		"dashboard.range.30":       "30 days",
		"dashboard.custom":         "Custom",
		"dashboard.total":          "Total",
		"dashboard.avg":            "Daily average",
		"dashboard.max":            "Longest day",
		"dashboard.topApp":         "Most used",
		"dashboard.topApps":        "Top apps",
		"dashboard.more":           "Other apps",
		"dashboard.nodata":         "No data in this period.",
		"dashboard.nousers":        "No activity recorded yet.",
		"theme.toggle":             "Toggle theme",
		"login.welcome":            "Sign in to manage screen time",

		// General
		"until.unlimited": "unlimited",
	},
	"de": {
		// Nav
		"nav.users":    "Benutzer",
		"nav.activity": "Aktivität",
		"nav.logout":   "Abmelden",

		// Login
		"login.username": "Benutzername",
		"login.password": "Passwort",
		"login.submit":   "Anmelden",
		"login.error":    "Ungültige Anmeldedaten",

		// Index
		"users.add":         "Benutzer hinzufügen",
		"users.placeholder": "Linux-Benutzername",
		"users.submit":      "Hinzufügen",

		// User list
		"user.locked":         "Gesperrt bis",
		"user.bonus":          "Bonus bis",
		"user.delete":         "Löschen",
		"user.delete.confirm": "Benutzer wirklich löschen",
		"user.lock":           "Sperren",
		"user.unlock":         "Entsperren",
		"user.bonus.btn":      "Bonus",

		// Schedules
		"schedule.days":   "Tage",
		"schedule.from":   "Von",
		"schedule.to":     "Bis",
		"schedule.delete": "Löschen",
		"schedule.none":   "Keine Zeitfenster konfiguriert. Benutzer ist nicht eingeschränkt.",
		"schedule.add":    "Zeitfenster hinzufügen",
		"schedule.submit": "Hinzufügen",

		// Days
		"day.mon": "Mo",
		"day.tue": "Di",
		"day.wed": "Mi",
		"day.thu": "Do",
		"day.fri": "Fr",
		"day.sat": "Sa",
		"day.sun": "So",

		// Activity
		"activity.screentime":     "Bildschirmzeit",
		"activity.screentime.tip": "Zeit in der der Benutzer aktiv angemeldet war (nicht idle oder gesperrt)",
		"activity.cattotal.tip":   "Summe aller App-Laufzeiten in dieser Kategorie (Apps können parallel laufen)",
		"activity.other":          "Sonstiges",
		"activity.nodata":         "Keine Aktivitätsdaten für diesen Tag.",

		// Dashboard
		"nav.dashboard":        "Dashboard",
		"dashboard.user":       "Benutzer",
		"dashboard.from":       "Von",
		"dashboard.to":         "Bis",
		"dashboard.update":     "Aktualisieren",
		"dashboard.screentime": "Bildschirmzeit pro Tag",
		"dashboard.apptime":    "App-Nutzung pro Tag",
		"dashboard.filterApp":  "App filtern",
		"dashboard.allApps":    "Alle Apps",

		// Redesign
		"state.locked":             "Gesperrt",
		"state.bonus":              "Bonuszeit",
		"state.free":               "Uneingeschränkt",
		"state.allowed":            "Gerade erlaubt",
		"state.blocked":            "Außerhalb der Zeiten",
		"status.online":            "Online",
		"status.idle":              "Inaktiv",
		"status.offline":           "Offline",
		"status.never":             "Noch nicht gesehen",
		"user.today":               "Heute",
		"user.until":               "bis",
		"user.bonus.add":           "Bonuszeit geben",
		"user.bonus.custom":        "Min.",
		"user.schedule":            "Zeitplan",
		"user.noactivity":          "Heute noch keine Aktivität",
		"users.empty":              "Noch keine Benutzer. Füge einen Linux-Benutzer hinzu, um loszulegen.",
		"schedule.preset.weekdays": "Mo–Fr",
		"schedule.preset.weekend":  "Wochenende",
		"schedule.preset.all":      "Täglich",
		"date.today":               "Heute",
		"date.yesterday":           "Gestern",
		"weekday.mon":              "Montag",
		"weekday.tue":              "Dienstag",
		"weekday.wed":              "Mittwoch",
		"weekday.thu":              "Donnerstag",
		"weekday.fri":              "Freitag",
		"weekday.sat":              "Samstag",
		"weekday.sun":              "Sonntag",
		"activity.apps":            "Apps",
		"activity.categories":      "Kategorien",
		"cat.Game":                 "Spiele",
		"cat.Network":              "Internet",
		"cat.Education":            "Lernen",
		"cat.Office":               "Büro",
		"cat.AudioVideo":           "Medien",
		"cat.Graphics":             "Grafik",
		"cat.Development":          "Entwicklung",
		"cat.Utility":              "Werkzeuge",
		"cat.System":               "System",
		"cat.Settings":             "Einstellungen",
		"dashboard.range.7":        "7 Tage",
		"dashboard.range.14":       "14 Tage",
		"dashboard.range.30":       "30 Tage",
		"dashboard.custom":         "Eigener",
		"dashboard.total":          "Gesamt",
		"dashboard.avg":            "Ø pro Tag",
		"dashboard.max":            "Längster Tag",
		"dashboard.topApp":         "Meistgenutzt",
		"dashboard.topApps":        "Top-Apps",
		"dashboard.more":           "Weitere Apps",
		"dashboard.nodata":         "Keine Daten in diesem Zeitraum.",
		"dashboard.nousers":        "Noch keine Aktivität aufgezeichnet.",
		"theme.toggle":             "Design wechseln",
		"login.welcome":            "Anmelden, um Bildschirmzeit zu verwalten",

		// General
		"until.unlimited": "unbegrenzt",
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
