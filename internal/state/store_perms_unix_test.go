//go:build !windows

package state

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSecretFilePerms asserts the state file is written 0600. On Windows the
// mode bits are not enforced by the OS, so this check is Unix-only.
func TestSecretFilePerms(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSecret("quadzero", "aetherpak", "sekrit-uuid"); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(dir, "quadzero__aetherpak.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Errorf("state file perm = %o, want 0600", perm)
	}
}
