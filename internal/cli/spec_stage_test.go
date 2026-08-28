package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareSpecDirStripsAnnotation(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "pkg.spec")
	content := "Name: pkg\nVersion: 1.0 # x-release-please-version\nRelease: 1\n"
	if err := os.WriteFile(spec, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	buildDir, err := prepareSpecDir(spec)
	if err != nil {
		t.Fatal(err)
	}
	if buildDir == dir {
		t.Fatalf("expected a staged dir, got original")
	}
	defer os.RemoveAll(buildDir)
	data, err := os.ReadFile(filepath.Join(buildDir, "pkg.spec"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "x-release-please-version") {
		t.Errorf("annotation not stripped: %q", string(data))
	}
	if !strings.Contains(string(data), "Version: 1.0") {
		t.Errorf("version line mangled: %q", string(data))
	}
	// The original spec must be untouched.
	orig, _ := os.ReadFile(spec)
	if !strings.Contains(string(orig), "x-release-please-version") {
		t.Errorf("original spec was modified")
	}
}

func TestPrepareSpecDirNoAnnotation(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "pkg.spec")
	if err := os.WriteFile(spec, []byte("Name: pkg\nVersion: 1.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	buildDir, err := prepareSpecDir(spec)
	if err != nil {
		t.Fatal(err)
	}
	if buildDir != dir {
		t.Errorf("expected original dir for clean spec, got %q", buildDir)
	}
}
