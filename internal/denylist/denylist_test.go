package denylist

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadNonExistent(t *testing.T) {
	users, err := Read(filepath.Join(t.TempDir(), "deny-users"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected empty list, got %v", users)
	}
}

func TestWriteAndRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deny-users")

	if err := Write(path, []string{"user2", "user1"}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	users, err := Read(path)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Should be sorted
	if len(users) != 2 || users[0] != "user1" || users[1] != "user2" {
		t.Fatalf("expected [user1 user2], got %v", users)
	}
}

func TestWriteEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deny-users")

	if err := Write(path, nil); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "" {
		t.Fatalf("expected empty file, got %q", string(data))
	}
}

func TestSyncAddsNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deny-users")
	Write(path, []string{"user1"})

	added, err := Sync(path, []string{"user1", "user2"})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(added) != 1 || added[0] != "user2" {
		t.Fatalf("expected [user2] added, got %v", added)
	}

	users, _ := Read(path)
	if len(users) != 2 {
		t.Fatalf("expected 2 users in file, got %d", len(users))
	}
}

func TestSyncRemovesStale(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deny-users")
	Write(path, []string{"user1", "user2"})

	added, err := Sync(path, []string{"user1"})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if len(added) != 0 {
		t.Fatalf("expected no additions, got %v", added)
	}

	users, _ := Read(path)
	if len(users) != 1 || users[0] != "user1" {
		t.Fatalf("expected [user1], got %v", users)
	}
}
