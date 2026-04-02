package server

import (
	"embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/BorisWilhelms/parentald/internal/config"
)

//go:embed all:templates
var templateFS embed.FS

var funcMap = template.FuncMap{
	"join": strings.Join,
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
}

func parseTemplates() *template.Template {
	return template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"),
	)
}
