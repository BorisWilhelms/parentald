package activity

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

var defaultDesktopDirs = []string{
	"/usr/share/applications",
	"/usr/local/share/applications",
}

// DesktopLookup maps executable basenames to application info from .desktop files.
type DesktopLookup struct {
	apps map[string]AppInfo
}

// NewDesktopLookup scans standard .desktop file locations and builds a lookup table.
func NewDesktopLookup() *DesktopLookup {
	dl := &DesktopLookup{apps: make(map[string]AppInfo)}
	for _, dir := range defaultDesktopDirs {
		dl.scanDir(dir)
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

	var name, execField, categories string
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

	info := AppInfo{Name: name}
	if categories != "" {
		cat := firstCategory(categories)
		if cat != "" {
			info.Category = &cat
		}
	}

	dl.apps[exeBasename] = info
}

func parseDesktopLine(line string) (key, value string, ok bool) {
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}
	return strings.TrimSpace(line[:idx]), strings.TrimSpace(line[idx+1:]), true
}

// parseExecField extracts the executable basename from a .desktop Exec= value.
// Handles: "env VAR=val /usr/bin/foo %u", "/usr/bin/foo --flag", "foo %F"
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

	return filepath.Base(fields[i])
}

// firstCategory returns the first non-empty category from a semicolon-separated list.
func firstCategory(categories string) string {
	for _, cat := range strings.Split(categories, ";") {
		cat = strings.TrimSpace(cat)
		if cat != "" {
			return cat
		}
	}
	return ""
}
