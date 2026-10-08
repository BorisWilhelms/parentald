package server

import (
	"embed"
	"fmt"
	"hash/fnv"
	"html/template"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/BorisWilhelms/parentald/internal/activity"
	"github.com/BorisWilhelms/parentald/internal/config"
)

//go:embed all:templates
var templateFS embed.FS

var funcMap = template.FuncMap{
	"join": strings.Join,
	"t":    t,
	"safeURL": func(s string) template.URL {
		return template.URL(s)
	},
	"dict": func(pairs ...any) map[string]any {
		m := make(map[string]any, len(pairs)/2)
		for i := 0; i < len(pairs)-1; i += 2 {
			m[pairs[i].(string)] = pairs[i+1]
		}
		return m
	},
	"isLocked": func(u config.User) bool {
		return u.LockedUntil != nil && time.Now().Before(*u.LockedUntil)
	},
	"hasBonus": func(u config.User) bool {
		return u.BonusUntil != nil && time.Now().Before(*u.BonusUntil)
	},
	"isInSchedule": func(u config.User) bool {
		return config.IsInSchedule(u.Schedules, time.Now())
	},
	"formatUntil": func(lang string, tt *time.Time) string {
		if tt == nil {
			return ""
		}
		if tt.Year() >= 9999 {
			return t(lang, "until.unlimited")
		}
		return fmt.Sprintf("%s %s", tt.Format("02.01."), tt.Format("15:04"))
	},
	"formatDuration": func(seconds int) string {
		h := seconds / 3600
		m := (seconds % 3600) / 60
		if h > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dm", m)
	},
	"prevDate": func(date string) string {
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return date
		}
		return t.AddDate(0, 0, -1).Format("2006-01-02")
	},
	"nextDate": func(date string) string {
		t, err := time.Parse("2006-01-02", date)
		if err != nil {
			return date
		}
		return t.AddDate(0, 0, 1).Format("2006-01-02")
	},
	"userState":    userState,
	"weekDays":     func() []string { return weekDays },
	"todayKey":     func() string { return weekDays[(int(time.Now().Weekday())+6)%7] },
	"nowPct":       func() float64 { n := time.Now(); return float64(n.Hour()*60+n.Minute()) / 14.4 },
	"scheduleBars": scheduleBars,
	"catName":      catName,
	"catColor":     catColor,
	"sortedCats":   sortedCats,
	"sortedApps":   sortedApps,
	"maxApp":       maxApp,
	"pct": func(a, b int) float64 {
		if b <= 0 {
			return 0
		}
		return float64(a) * 100 / float64(b)
	},
	"initial": func(name string) string {
		for _, r := range name {
			return string(unicode.ToUpper(r))
		}
		return "?"
	},
	"avatarHue": func(name string) int {
		h := fnv.New32a()
		h.Write([]byte(name))
		return int(h.Sum32() % 360)
	},
	"dateLabel": dateLabel,
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"isToday": func(date string) bool { return date >= time.Now().Format("2006-01-02") },
	"sortedKeys": func(m map[string]any) []string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	},
}

func parseTemplates() *template.Template {
	return template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"),
	)
}

var weekDays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

// userState mirrors config.IsAllowed priority: lock > bonus > no schedules > schedule.
func userState(u config.User) string {
	now := time.Now()
	switch {
	case u.LockedUntil != nil && now.Before(*u.LockedUntil):
		return "locked"
	case u.BonusUntil != nil && now.Before(*u.BonusUntil):
		return "bonus"
	case len(u.Schedules) == 0:
		return "free"
	case config.IsInSchedule(u.Schedules, now):
		return "allowed"
	}
	return "blocked"
}

type scheduleBar struct {
	Left, Width float64
	Label       string
}

// scheduleBars returns the allowed windows for a day as percentages of 24h.
func scheduleBars(schedules []config.Schedule, day string) []scheduleBar {
	var bars []scheduleBar
	for _, s := range schedules {
		found := false
		for _, d := range s.Days {
			if d == day {
				found = true
			}
		}
		if !found {
			continue
		}
		from, err1 := minutesOf(s.From)
		to, err2 := minutesOf(s.To)
		if err1 != nil || err2 != nil || to <= from {
			continue
		}
		bars = append(bars, scheduleBar{
			Left:  float64(from) / 14.4,
			Width: float64(to-from) / 14.4,
			Label: s.From + "–" + s.To,
		})
	}
	return bars
}

func minutesOf(hhmm string) (int, error) {
	var h, m int
	if _, err := fmt.Sscanf(hhmm, "%d:%d", &h, &m); err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

// catName translates a freedesktop category, falling back to the raw key.
func catName(lang, cat string) string {
	key := "cat." + cat
	if v := t(lang, key); v != key {
		return v
	}
	return cat
}

// categoryOrder fixes the color slot per category so a category keeps its color everywhere.
var categoryOrder = []string{"Game", "Network", "Education", "Office", "AudioVideo", "Graphics", "Development", "Utility"}

func catColor(cat string) template.CSS {
	for i, c := range categoryOrder {
		if c == cat {
			return template.CSS(fmt.Sprintf("var(--c%d)", i+1))
		}
	}
	return "var(--c-other)"
}

// sortedCats returns the categories of a user's day, largest first, without "Other".
func sortedCats(ua *activity.UserActivity) []string {
	var cats []string
	for c := range ua.CategoryTotals {
		if c != "Other" {
			cats = append(cats, c)
		}
	}
	sort.Slice(cats, func(i, j int) bool {
		return ua.CategoryTotals[cats[i]] > ua.CategoryTotals[cats[j]]
	})
	return cats
}

// sortedApps returns all categorized apps of a user's day, largest first.
func sortedApps(ua *activity.UserActivity) []*activity.AppTime {
	var apps []*activity.AppTime
	for _, a := range ua.Apps {
		if a.Category != nil {
			apps = append(apps, a)
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		if apps[i].Seconds != apps[j].Seconds {
			return apps[i].Seconds > apps[j].Seconds
		}
		return apps[i].Name < apps[j].Name
	})
	return apps
}

func maxApp(ua *activity.UserActivity) int {
	m := 0
	for _, a := range ua.Apps {
		if a.Seconds > m {
			m = a.Seconds
		}
	}
	return m
}

// dateLabel renders "Today", "Yesterday" or a weekday with date.
func dateLabel(lang, date string) string {
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	today := time.Now().Format("2006-01-02")
	switch date {
	case today:
		return t(lang, "date.today")
	case time.Now().AddDate(0, 0, -1).Format("2006-01-02"):
		return t(lang, "date.yesterday")
	}
	return t(lang, "weekday."+weekDays[(int(d.Weekday())+6)%7]) + ", " + d.Format("02.01.2006")
}
