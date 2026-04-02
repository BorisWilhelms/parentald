package server

import (
	"embed"
	"html/template"
	"strings"
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
}

func parseTemplates() *template.Template {
	return template.Must(
		template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"),
	)
}
