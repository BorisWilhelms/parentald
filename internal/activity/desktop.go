package activity

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

var defaultDesktopDirs = []string{
	"/usr/share/applications",
	"/usr/local/share/applications",
	"/var/lib/flatpak/exports/share/applications",
}

// DesktopLookup maps executable basenames to application info from .desktop files.
type DesktopLookup struct {
	apps map[string]AppInfo
}

// NewDesktopLookup scans standard .desktop file locations and builds a lookup table.
// usernames are used to find per-user flatpak installs.
func NewDesktopLookup(usernames []string) *DesktopLookup {
	dl := &DesktopLookup{apps: make(map[string]AppInfo)}
	for _, dir := range defaultDesktopDirs {
		dl.scanDir(dir)
	}
	for _, username := range usernames {
		u, err := user.Lookup(username)
		if err != nil {
			continue
		}
		dl.scanDir(filepath.Join(u.HomeDir, ".local", "share", "flatpak", "exports", "share", "applications"))
		dl.scanDir(filepath.Join(u.HomeDir, ".local", "share", "applications"))
	}
	return dl
}

// Lookup returns the AppInfo for the given executable basename.
func (dl *DesktopLookup) Lookup(exeBasename string) (AppInfo, bool) {
	info, ok := dl.apps[exeBasename]
	return info, ok
}

func (dl *DesktopLookup) scanDir(dir string) {
	entries, err := filepath.Glob(filepath.Join(dir, "*.desktop"))
	if err != nil {
		return
	}
	for _, path := range entries {
		dl.parseFile(path)
	}
}

func (dl *DesktopLookup) parseFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	var name, execField, categories, iconName string
	inDesktopEntry := false

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[Desktop Entry]" {
			inDesktopEntry = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			// New section — stop reading
			if inDesktopEntry {
				break
			}
			continue
		}
		if !inDesktopEntry {
			continue
		}

		if k, v, ok := parseDesktopLine(line); ok {
			switch k {
			case "Name":
				if name == "" { // only take first Name (unlocalized)
					name = v
				}
			case "Exec":
				execField = v
			case "Categories":
				categories = v
			case "Icon":
				iconName = v
			}
		}
	}

	if execField == "" || name == "" {
		return
	}

	exeBasename := parseExecField(execField)
	if exeBasename == "" {
		return
	}

	// Skip Steam game shortcuts (Exec=steam steam://rungameid/...) — these would
	// overwrite the Steam app entry since they share the same executable.
	if exeBasename == "steam" && strings.Contains(execField, "steam://") {
		return
	}

	info := AppInfo{Name: name}
	if categories != "" {
		cat := firstCategory(categories)
		if cat != "" {
			info.Category = &cat
		}
	}
	if iconName != "" {
		if dataURI := resolveIcon(iconName); dataURI != "" {
			info.Icon = &dataURI
		}
	}

	// Don't overwrite — first entry wins (prevents Steam games from
	// overwriting Steam itself, as games have Exec=steam steam://rungameid/...)
	if _, exists := dl.apps[exeBasename]; !exists {
		dl.apps[exeBasename] = info
	}
}

func parseDesktopLine(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// parseExecField extracts the executable basename from a .desktop Exec= value.
// Handles: "env VAR=val /usr/bin/foo %u", "/usr/bin/foo --flag", "foo %F",
// and flatpak: "/usr/bin/flatpak run --branch=stable com.example.App"
func parseExecField(exec string) string {
	fields := strings.Fields(exec)

	// Skip leading env assignments (e.g., "env VAR=val" or "VAR=val")
	i := 0
	for i < len(fields) {
		if fields[i] == "env" {
			i++
			continue
		}
		if strings.Contains(fields[i], "=") && !strings.HasPrefix(fields[i], "/") && !strings.HasPrefix(fields[i], "-") {
			i++
			continue
		}
		break
	}

	if i >= len(fields) {
		return ""
	}

	cmd := filepath.Base(fields[i])

	// Handle "flatpak run [options] com.example.App [args]"
	if cmd == "flatpak" {
		return parseFlatpakExec(fields[i+1:])
	}

	return cmd
}

// parseFlatpakExec extracts the app ID from a "flatpak run" command.
// Returns the last component of the app ID (e.g., "Steam" from "com.valvesoftware.Steam").
func parseFlatpakExec(args []string) string {
	foundRun := false
	for _, arg := range args {
		if arg == "run" {
			foundRun = true
			continue
		}
		if !foundRun {
			continue
		}
		// Skip flags
		if strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "%") {
			continue
		}
		// This should be the app ID (e.g., "com.valvesoftware.Steam")
		parts := strings.Split(arg, ".")
		if len(parts) >= 3 {
			return arg // return full app ID as lookup key
		}
	}
	return ""
}

// Icon search directories, in priority order.
var iconSearchDirs = []string{
	"/usr/share/icons/hicolor/48x48/apps",
	"/usr/share/icons/hicolor/64x64/apps",
	"/usr/share/icons/hicolor/scalable/apps",
	"/usr/share/icons/hicolor/256x256/apps",
	"/usr/share/icons/hicolor/128x128/apps",
	"/usr/share/pixmaps",
	"/var/lib/flatpak/exports/share/icons/hicolor/64x64/apps",
	"/var/lib/flatpak/exports/share/icons/hicolor/48x48/apps",
	"/var/lib/flatpak/exports/share/icons/hicolor/scalable/apps",
	"/var/lib/flatpak/exports/share/icons/hicolor/128x128/apps",
}

// resolveIcon finds an icon file and returns it as a data URI.
// iconName can be an absolute path or a theme icon name (e.g., "firefox").
func resolveIcon(iconName string) string {
	// If it's an absolute path, read directly
	if filepath.IsAbs(iconName) {
		return readIconAsDataURI(iconName)
	}

	// Search in standard directories
	for _, dir := range iconSearchDirs {
		// Try exact name with extensions
		for _, ext := range []string{".png", ".svg", ".xpm"} {
			path := filepath.Join(dir, iconName+ext)
			if uri := readIconAsDataURI(path); uri != "" {
				return uri
			}
		}
		// Try exact name (might already have extension)
		path := filepath.Join(dir, iconName)
		if uri := readIconAsDataURI(path); uri != "" {
			return uri
		}
	}

	return ""
}

func readIconAsDataURI(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	// Limit icon size to 32KB to avoid bloating reports
	if len(data) > 32*1024 {
		return ""
	}

	mime := "image/png"
	if strings.HasSuffix(path, ".svg") {
		mime = "image/svg+xml"
	} else if strings.HasSuffix(path, ".xpm") {
		return "" // skip xpm, not useful for web
	}

	return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(data))
}

// mainCategories are the freedesktop.org Main Categories.
// These describe the app's purpose, not its toolkit or desktop.
var mainCategories = map[string]bool{
	"AudioVideo":  true,
	"Audio":       true,
	"Video":       true,
	"Development": true,
	"Education":   true,
	"Game":        true,
	"Graphics":    true,
	"Network":     true,
	"Office":      true,
	"Science":     true,
	"Settings":    true,
	"System":      true,
	"Utility":     true,
}

// firstCategory returns the first main category from a semicolon-separated list.
// Skips toolkit/desktop markers like GNOME, GTK, KDE, Qt.
// Falls back to the first non-empty category if no main category is found.
func firstCategory(categories string) string {
	var fallback string
	for _, cat := range strings.Split(categories, ";") {
		cat = strings.TrimSpace(cat)
		if cat == "" {
			continue
		}
		if mainCategories[cat] {
			return cat
		}
		if fallback == "" {
			fallback = cat
		}
	}
	return fallback
}
