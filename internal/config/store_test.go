package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_LoadNonExistent(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "config.json"))
	if err := s.Load(); err != nil {
		t.Fatalf("Load should not fail for missing file: %v", err)
	}
	cfg := s.Get()
	if cfg.Users == nil {
		t.Fatal("Users map should be initialized")
	}
	if len(cfg.Users) != 0 {
		t.Fatal("Users map should be empty")
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s := NewStore(path)
	s.Load()

	err := s.Update(func(cfg *Config) {
		cfg.Users["kind1"] = User{
			Schedules: []Schedule{
				{Days: []string{"mon"}, From: "14:00", To: "18:00"},
			},
		}
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Load into a new store
	s2 := NewStore(path)
	if err := s2.Load(); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	cfg := s2.Get()
	if len(cfg.Users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(cfg.Users))
	}
	if len(cfg.Users["kind1"].Schedules) != 1 {
		t.Fatal("expected 1 schedule for kind1")
	}
}

func TestStore_GetReturnsCopy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	s := NewStore(path)
	s.Load()

	s.Update(func(cfg *Config) {
		cfg.Users["kind1"] = User{Schedules: []Schedule{{Days: []string{"mon"}, From: "10:00", To: "12:00"}}}
	})

	cfg := s.Get()
	cfg.Users["kind1"] = User{} // mutate the copy

	// Original should be unchanged
	original := s.Get()
	if len(original.Users["kind1"].Schedules) != 1 {
		t.Error("Get should return a deep copy")
	}
}

func TestStore_LoadInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte("not json"), 0644)

	s := NewStore(path)
	if err := s.Load(); err == nil {
		t.Fatal("Load should fail for invalid JSON")
	}
}
