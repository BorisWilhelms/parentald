package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
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

// Save writes the current config to disk atomically.
func (s *Store) Save() error {
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
		users[k] = User{Schedules: schedules}
	}
	return Config{Users: users}
}

// Update applies a mutation function to the config and saves to disk.
func (s *Store) Update(fn func(*Config)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn(&s.cfg)
	return s.Save()
}
