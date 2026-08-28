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

func TestResolveInstructions(t *testing.T) {
	// Inline text passes through.
	if got, err := resolveInstructions("install with dnf"); err != nil || got != "install with dnf" {
		t.Errorf("inline = %q, %v", got, err)
	}
	// A file path is read.
	dir := t.TempDir()
	fp := filepath.Join(dir, "install.md")
	if err := os.WriteFile(fp, []byte("# install\nrun this"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := resolveInstructions(fp); err != nil || got != "# install\nrun this" {
		t.Errorf("file = %q, %v", got, err)
	}
}

func TestGithubRepoLinks(t *testing.T) {
	home, contact := githubRepoLinks("abn/coprctl")
	if home != "https://github.com/abn/coprctl" {
		t.Errorf("homepage = %q", home)
	}
	if contact != "https://github.com/abn/coprctl/issues" {
		t.Errorf("contact = %q", contact)
	}
	// Handles a full URL input too.
	home2, contact2 := githubRepoLinks("https://github.com/abn/coprctl")
	if home2 != "https://github.com/abn/coprctl" || contact2 != "https://github.com/abn/coprctl/issues" {
		t.Errorf("url input = %q, %q", home2, contact2)
	}
}
