package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveListOwner(t *testing.T) {
	// Set up a temp HOME with a legacy config carrying a username.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	legacy := filepath.Join(home, ".config", "copr")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	content := "[copr-cli]\nlogin = \"l\"\nusername = \"alice\"\ntoken = \"t\"\ncopr_url = \"https://copr.example.org\"\n"
	if err := os.WriteFile(legacy, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	// Explicit arg wins.
	if got := resolveListOwner(app, []string{"bob"}); got != "bob" {
		t.Errorf("explicit arg = %q, want bob", got)
	}
	// No arg defaults to the authenticated user.
	if got := resolveListOwner(app, []string{}); got != "alice" {
		t.Errorf("default owner = %q, want alice", got)
	}
}

func TestResolveListOwnerNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	app := NewApp()
	// No config and no arg: anonymous read, empty owner.
	if got := resolveListOwner(app, []string{}); got != "" {
		t.Errorf("no config owner = %q, want empty", got)
	}
}
