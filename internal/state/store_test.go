package state

import (
	"path/filepath"
	"testing"
)

func TestSecretRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetSecret("quadzero", "aetherpak", "sekrit-uuid"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetSecret("quadzero", "aetherpak")
	if err != nil {
		t.Fatal(err)
	}
	if got != "sekrit-uuid" {
		t.Errorf("secret = %q", got)
	}
}

func TestMissingSecret(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	if _, err := s.GetSecret("no", "such"); err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestHookID(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	if err := s.SetHookID("o", "p", 12345); err != nil {
		t.Fatal(err)
	}
	// SetHookID then read via a fresh store.
	s2, _ := NewStore(s.dir)
	if got, _ := s2.GetSecret("o", "p"); got != "" {
		t.Errorf("unexpected secret: %q", got)
	}
}
