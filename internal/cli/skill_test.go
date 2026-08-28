package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillInstallMutualExclusion(t *testing.T) {
	root := Root(NewApp())
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"skill", "install", "--global", "--target", "/tmp/skilltest"})
	err := root.ExecuteContext(t.Context())
	if err == nil {
		t.Fatal("expected error for --global with --target")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Errorf("error = %q, want mention of mutual exclusivity", err)
	}
}

func TestSkillInstallGlobal(t *testing.T) {
	// Point HOME at a temp dir so the global install target is isolated.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	root := Root(NewApp())
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"skill", "install", "--global"})
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"coprctl", "coprctl-debug"} {
		if _, err := os.Stat(filepath.Join(dir, name, "SKILL.md")); err != nil {
			t.Errorf("expected %s/SKILL.md, got %v", name, err)
		}
	}
}
