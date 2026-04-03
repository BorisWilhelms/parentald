package activity

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

// ScanUserProcesses returns the deduplicated set of executable basenames
// for all processes owned by the given user.
func ScanUserProcesses(username string) ([]string, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("lookup user %s: %w", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, fmt.Errorf("parse uid %s: %w", u.Uid, err)
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("read /proc: %w", err)
	}

	seen := make(map[string]bool)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		if !isOwnedBy(pid, uid) {
			continue
		}

		exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			continue // kernel thread, zombie, or permission issue
		}

		// Skip deleted binaries (e.g., "/usr/bin/foo (deleted)")
		exe = strings.TrimSuffix(exe, " (deleted)")
		basename := filepath.Base(exe)

		// Check if this is a flatpak process — use the app ID instead of bwrap/binary name
		if flatpakID := flatpakAppID(pid); flatpakID != "" {
			seen[flatpakID] = true
		} else {
			seen[basename] = true
		}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result, nil
}

// isOwnedBy checks if a process is owned by the given UID.
func isOwnedBy(pid, uid int) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				processUID, err := strconv.Atoi(fields[1])
				if err != nil {
					return false
				}
				return processUID == uid
			}
		}
	}
	return false
}

// flatpakAppID reads /proc/<pid>/cgroup to detect flatpak processes.
// Returns the app ID (e.g., "com.valvesoftware.Steam") or empty string.
func flatpakAppID(pid int) string {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		// Look for: "0::/user.slice/.../app-flatpak-com.valvesoftware.Steam-12345.scope"
		if idx := strings.Index(line, "app-flatpak-"); idx >= 0 {
			rest := line[idx+len("app-flatpak-"):]
			// Strip the trailing instance ID and .scope
			if scopeIdx := strings.LastIndex(rest, "-"); scopeIdx > 0 {
				appID := rest[:scopeIdx]
				// Unescape: flatpak uses \x2d for hyphens in cgroup names
				appID = strings.ReplaceAll(appID, `\x2d`, "-")
				return appID
			}
		}
	}
	return ""
}

// IsSessionActive checks if the user has at least one active (not idle, not locked) session.
// Returns true if loginctl is not available (fail-open for tracking).
func IsSessionActive(username string) bool {
	// Get session IDs for the user
	out, err := exec.Command("loginctl", "show-user", username, "--property=Sessions").CombinedOutput()
	if err != nil {
		return true // fail-open
	}

	line := strings.TrimSpace(string(out))
	// Format: "Sessions=1 5" or "Sessions="
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 || parts[1] == "" {
		return false // no sessions
	}

	sessions := strings.Fields(parts[1])
	for _, sid := range sessions {
		if isSessionActiveByID(sid) {
			return true
		}
	}
	return false
}

func isSessionActiveByID(sessionID string) bool {
	out, err := exec.Command("loginctl", "show-session", sessionID,
		"--property=IdleHint", "--property=LockedHint").CombinedOutput()
	if err != nil {
		return true // fail-open
	}

	idle := false
	locked := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "IdleHint=yes" {
			idle = true
		}
		if line == "LockedHint=yes" {
			locked = true
		}
	}

	return !idle && !locked
}
