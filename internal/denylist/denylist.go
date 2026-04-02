package denylist

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Read returns the list of denied usernames from the file.
// Returns an empty list if the file does not exist.
func Read(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var users []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			users = append(users, line)
		}
	}
	return users, nil
}

// Write atomically writes the list of denied usernames to the file.
func Write(path string, users []string) error {
	sort.Strings(users)

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, "deny-users-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	content := strings.Join(users, "\n")
	if len(users) > 0 {
		content += "\n"
	}

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0644); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, path)
}

// Sync updates the deny file to match the given set of denied users.
// Returns the list of users that were newly added (for session termination).
func Sync(path string, denied []string) (added []string, err error) {
	current, err := Read(path)
	if err != nil {
		return nil, err
	}

	currentSet := make(map[string]bool, len(current))
	for _, u := range current {
		currentSet[u] = true
	}

	for _, u := range denied {
		if !currentSet[u] {
			added = append(added, u)
		}
	}

	return added, Write(path, denied)
}
