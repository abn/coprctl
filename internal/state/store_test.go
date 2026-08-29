package state

import (
	"encoding/json"
	"os"
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
	if rec := readRecord(t, s2, "o", "p"); rec.HookID != 12345 {
		t.Errorf("hook id = %d, want 12345", rec.HookID)
	}
}

func TestSetSecretPreservesHookID(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	if err := s.SetHookID("o", "p", 12345); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSecret("o", "p", "sekrit"); err != nil {
		t.Fatal(err)
	}
	rec := readRecord(t, s, "o", "p")
	if rec.HookID != 12345 {
		t.Errorf("hook id = %d, want 12345", rec.HookID)
	}
	if rec.WebhookSecret != "sekrit" {
		t.Errorf("secret = %q, want sekrit", rec.WebhookSecret)
	}
}

func TestSetHookIDPreservesSecret(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	if err := s.SetSecret("o", "p", "sekrit"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHookID("o", "p", 12345); err != nil {
		t.Fatal(err)
	}
	rec := readRecord(t, s, "o", "p")
	if rec.HookID != 12345 {
		t.Errorf("hook id = %d, want 12345", rec.HookID)
	}
	if rec.WebhookSecret != "sekrit" {
		t.Errorf("secret = %q, want sekrit", rec.WebhookSecret)
	}
}

func TestCorruptRecordErrors(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	path := s.recordPath("o", "p")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.SetSecret("o", "p", "sekrit"); err == nil {
		t.Fatal("expected error for corrupt state file")
	}
	if err := s.SetHookID("o", "p", 1); err == nil {
		t.Fatal("expected error for corrupt state file")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{not json" {
		t.Errorf("corrupt file was overwritten: %q", data)
	}
}

func TestSetHookIDMissingRecord(t *testing.T) {
	s, _ := NewStore(t.TempDir())
	if err := s.SetHookID("o", "p", 12345); err != nil {
		t.Fatal(err)
	}
	if rec := readRecord(t, s, "o", "p"); rec.HookID != 12345 {
		t.Errorf("hook id = %d, want 12345", rec.HookID)
	}
}

func readRecord(t *testing.T, s *Store, owner, project string) integrationRecord {
	t.Helper()
	data, err := os.ReadFile(s.recordPath(owner, project))
	if err != nil {
		t.Fatal(err)
	}
	var rec integrationRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	return rec
}
