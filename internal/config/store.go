package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store manages reading and writing the config JSON file.
type Store struct {
	path string
	mu   sync.RWMutex
	cfg  Config
}

// NewStore creates a new Store for the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

// Load reads the config from disk. If the file does not exist, an empty config is initialized.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.cfg = Config{Users: make(map[string]User)}
		return nil
	}
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if cfg.Users == nil {
		cfg.Users = make(map[string]User)
	}
	s.cfg = cfg
	return nil
}

// save writes the current config to disk atomically.
// Must be called while holding the write lock.
func (s *Store) save() error {
	data, err := json.MarshalIndent(s.cfg, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "parentald-config-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}

	return os.Rename(tmpName, s.path)
}

// Get returns a copy of the current config.
func (s *Store) Get() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Deep copy the users map
	users := make(map[string]User, len(s.cfg.Users))
	for k, v := range s.cfg.Users {
		schedules := make([]Schedule, len(v.Schedules))
		copy(schedules, v.Schedules)
		u := User{Schedules: schedules}
		if v.LockedUntil != nil {
			t := *v.LockedUntil
			u.LockedUntil = &t
		}
		if v.BonusUntil != nil {
			t := *v.BonusUntil
			u.BonusUntil = &t
		}
		users[k] = u
	}
	return Config{Users: users}
}

// Update applies a mutation function to the config and saves to disk.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn(&s.cfg)
	s.cleanupExpired(time.Now())
	return s.save()
}

// cleanupExpired clears LockedUntil and BonusUntil fields that are in the past.
// Must be called while holding the write lock.
func (s *Store) cleanupExpired(now time.Time) {
	for k, u := range s.cfg.Users {
		changed := false
		if u.LockedUntil != nil && now.After(*u.LockedUntil) {
			u.LockedUntil = nil
			changed = true
		}
		if u.BonusUntil != nil && now.After(*u.BonusUntil) {
			u.BonusUntil = nil
			changed = true
		}
		if changed {
			s.cfg.Users[k] = u
		}
	}
}
