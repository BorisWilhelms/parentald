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

type processInfo struct {
	pid      int
	ppid     int
	exe      string
	children []*processInfo
}

// ScanUserProcesses returns the deduplicated set of top-level app names
// for all processes owned by the given user. It builds a process tree and
// returns only direct children of session roots (e.g., children of
// "systemd --user"), collapsing all descendant processes into their
// top-level ancestor.
func ScanUserProcesses(username string) ([]string, error) {
	u, err := user.Lookup(username)
	if err != nil {
		return nil, fmt.Errorf("lookup user %s: %w", username, err)
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return nil, fmt.Errorf("parse uid %s: %w", u.Uid, err)
	}

	tree := buildProcessTree(uid)
	return findTopLevelApps(tree, uid), nil
}

// buildProcessTree scans /proc and builds a tree of processes owned by the given UID.
func buildProcessTree(uid int) map[int]*processInfo {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}

	procs := make(map[int]*processInfo)

	// First pass: collect all user processes
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		procUID, ppid := readUIDAndPPID(pid)
		if procUID != uid {
			continue
		}

		exe, err := os.Readlink(fmt.Sprintf("/proc/%d/exe", pid))
		if err != nil {
			continue
		}
		exe = strings.TrimSuffix(exe, " (deleted)")

		procs[pid] = &processInfo{
			pid:  pid,
			ppid: ppid,
			exe:  filepath.Base(exe),
		}
	}

	// Second pass: build parent-child relationships
	for _, proc := range procs {
		if parent, ok := procs[proc.ppid]; ok {
			parent.children = append(parent.children, proc)
		}
	}

	return procs
}

// findTopLevelApps finds session roots and returns their direct children's exe names.
// A session root is a user process whose parent is NOT owned by the same user.
func findTopLevelApps(procs map[int]*processInfo, uid int) []string {
	seen := make(map[string]bool)

	for _, proc := range procs {
		// Session root: parent is not in our process map (not owned by this user)
		if _, parentIsOurs := procs[proc.ppid]; parentIsOurs {
			continue
		}

		// This is a session root — collect its direct children as top-level apps
		for _, child := range proc.children {
			name := resolveAppName(child)
			seen[name] = true
		}
	}

	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	return result
}

// resolveAppName determines the app name for a top-level process.
// Checks the process and all descendants for flatpak cgroup markers.
func resolveAppName(proc *processInfo) string {
	if flatpakID := findFlatpakInSubtree(proc); flatpakID != "" {
		return flatpakID
	}
	return proc.exe
}

// findFlatpakInSubtree checks a process and all its descendants for a flatpak app ID.
func findFlatpakInSubtree(proc *processInfo) string {
	if id := flatpakAppID(proc.pid); id != "" {
		return id
	}
	for _, child := range proc.children {
		if id := findFlatpakInSubtree(child); id != "" {
			return id
		}
	}
	return ""
}

// readUIDAndPPID reads the UID and PPID from /proc/<pid>/status.
func readUIDAndPPID(pid int) (uid, ppid int) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return -1, -1
	}
	uid = -1
	ppid = -1
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "Uid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				uid, _ = strconv.Atoi(fields[1])
			}
		}
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				ppid, _ = strconv.Atoi(fields[1])
			}
		}
	}
	return uid, ppid
}

// isOwnedBy checks if a process is owned by the given UID.
func isOwnedBy(pid, uid int) bool {
	procUID, _ := readUIDAndPPID(pid)
	return procUID == uid
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
