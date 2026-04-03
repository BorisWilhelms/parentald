package server

import (
	"embed"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"github.com/BorisWilhelms/parentald/internal/config"
)

//go:embed all:templates
var templateFS embed.FS

var funcMap = template.FuncMap{
	"join": strings.Join,
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
	"formatUntil": func(t *time.Time) string {
		if t == nil {
			return ""
		}
		if t.Year() >= 9999 {
			return "unbegrenzt"
		}
		return fmt.Sprintf("%s %s", t.Format("02.01."), t.Format("15:04"))
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
