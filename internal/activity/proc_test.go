package activity

import (
	"os"
	"testing"
)

func TestIsOwnedBy(t *testing.T) {
	pid := os.Getpid()
	uid := os.Getuid()

	if !isOwnedBy(pid, uid) {
		t.Errorf("current process (PID %d) should be owned by UID %d", pid, uid)
	}
	if isOwnedBy(pid, uid+99999) {
		t.Error("should not match wrong UID")
	}
}

func TestIsOwnedBy_InvalidPID(t *testing.T) {
	if isOwnedBy(-1, 0) {
		t.Error("invalid PID should return false")
	}
}
