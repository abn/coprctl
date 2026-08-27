package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSpec(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "aetherpak.spec")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectSpecFields(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, `Name: aetherpak
Version: 0.4.1
Summary: Self-hosting toolchain
URL: https://github.com/quadzero/aetherpak
License: MIT
Source0: https://example.com/aetherpak-0.4.1.tar.gz

%prep
go mod download
`)
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(res.Specs))
	}
	s := res.Specs[0]
	if s.Name != "aetherpak" || s.Version != "0.4.1" || s.License != "MIT" {
		t.Errorf("spec fields wrong: %+v", s)
	}
	if !s.Source0IsURL {
		t.Error("expected Source0 to be detected as URL")
	}
	if !s.NetworkInBuild {
		t.Error("expected network call to be detected")
	}
}

func TestDetectNoGit(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "Name: foo\nVersion: 1.0\nLicense: MIT\n")
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.CloneURL != "" {
		t.Errorf("expected no clone url without git, got %q", res.CloneURL)
	}
	// Chroots always require a decision.
	if len(res.Decisions) != 1 || res.Decisions[0].Flag != "--chroot" {
		t.Errorf("expected chroot decision, got %+v", res.Decisions)
	}
}

func TestDetectTitoMethod(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "Name: foo\nLicense: MIT\n")
	if err := os.MkdirAll(filepath.Join(dir, ".tito"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasTito {
		t.Fatal("expected tito detection")
	}
	if res.Specs[0].Method != "tito" {
		t.Errorf("method = %q, want tito", res.Specs[0].Method)
	}
}

func TestDetectGitRemote(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `[remote "origin"]
	url = https://github.com/quadzero/aetherpak.git
	fetch = +refs/heads/*:refs/remotes/origin/*
`
	if err := os.WriteFile(filepath.Join(dir, ".git", "config"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, dir, "Name: aetherpak\nLicense: MIT\n")
	res, err := Detect(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.Forge != "github" || res.RepoName != "aetherpak" {
		t.Errorf("forge=%q repo=%q", res.Forge, res.RepoName)
	}
	if res.CloneURL != "https://github.com/quadzero/aetherpak.git" {
		t.Errorf("clone url = %q", res.CloneURL)
	}
	if res.Proposed == nil || len(res.Proposed.Spec.Packages) != 1 {
		t.Fatalf("proposed manifest wrong: %+v", res.Proposed)
	}
}

func TestDetectSpecSubdirectorySplit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "rpm"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSpec(t, filepath.Join(dir, "rpm"), "Name: hello\nLicense: MIT\n")
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	pkg := res.Proposed.Spec.Packages[0]
	if pkg.Source.Spec != "aetherpak.spec" {
		t.Errorf("spec = %q, want basename", pkg.Source.Spec)
	}
	if pkg.Source.Subdirectory != "rpm" {
		t.Errorf("subdirectory = %q, want rpm", pkg.Source.Subdirectory)
	}
}

func TestDetectRecordsInferredPaths(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "Name: hello\nLicense: MIT\n")
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Proposed.XCoprctl.Inferred) == 0 {
		t.Fatal("expected inferred paths to be recorded")
	}
}
