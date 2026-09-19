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

func writeSpecTemplate(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectSpecTemplateInPackagingDir(t *testing.T) {
	dir := t.TempDir()
	writeSpecTemplate(t, filepath.Join(dir, "packaging", "rpm"), "aetherpak.spec.in",
		"Name: aetherpak\nVersion: @RPM_VERSION@\nSummary: Self-hosting toolchain\nLicense: MIT\n")
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(res.Specs))
	}
	s := res.Specs[0]
	if !s.Template {
		t.Error("expected template flag on .spec.in")
	}
	if !s.Uncertain {
		t.Error("expected uncertain version placeholder on template")
	}
	if s.Method != "make_srpm" {
		t.Errorf("method = %q, want make_srpm", s.Method)
	}
	pkg := res.Proposed.Spec.Packages[0]
	if pkg.Source.Spec != "aetherpak.spec" {
		t.Errorf("spec = %q, want rendered basename", pkg.Source.Spec)
	}
	if pkg.Source.Subdirectory != "packaging/rpm" {
		t.Errorf("subdirectory = %q, want packaging/rpm", pkg.Source.Subdirectory)
	}
	found := false
	for _, d := range res.Decisions {
		if d.Field == "spec.makeSrpmTarget" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected makeSrpmTarget decision, got %+v", res.Decisions)
	}
}

func TestDetectTemplateWithCoprMakefileNeedsNoDecision(t *testing.T) {
	dir := t.TempDir()
	writeSpecTemplate(t, filepath.Join(dir, "packaging", "rpm"), "aetherpak.spec.in",
		"Name: aetherpak\nVersion: @RPM_VERSION@\nLicense: MIT\n")
	if err := os.MkdirAll(filepath.Join(dir, ".copr"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".copr", "Makefile"), []byte("srpm:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasCoprMake {
		t.Fatal("expected copr make detection")
	}
	for _, d := range res.Decisions {
		if d.Field == "spec.makeSrpmTarget" {
			t.Errorf("unexpected makeSrpmTarget decision when .copr/Makefile exists")
		}
	}
}

func TestDetectCargoSignals(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "Name: hello\nLicense: MIT\n")
	cargo := "[workspace]\n[workspace.package]\nversion = \"0.4.1\"\nrust-version = \"1.95\"\n"
	if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte(cargo), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasCargo {
		t.Error("expected cargo detection")
	}
	if res.RustVersion != "1.95" {
		t.Errorf("rust version = %q, want 1.95", res.RustVersion)
	}
}

func TestDetectCargoVersionSpellings(t *testing.T) {
	for _, cargo := range []string{
		"  rust-version = \"1.95\"\n",
		"rust-version = '1.95'\n",
		"\trust-version='1.95'\n",
	} {
		dir := t.TempDir()
		writeSpec(t, dir, "Name: hello\nLicense: MIT\n")
		if err := os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\n"+cargo), 0o644); err != nil {
			t.Fatal(err)
		}
		res, err := Detect(dir, false)
		if err != nil {
			t.Fatal(err)
		}
		if res.RustVersion != "1.95" {
			t.Errorf("cargo %q: rust version = %q, want 1.95", cargo, res.RustVersion)
		}
	}
}

func TestDetectTemplateOverridesTito(t *testing.T) {
	dir := t.TempDir()
	writeSpecTemplate(t, dir, "aetherpak.spec.in", "Name: aetherpak\nVersion: @RPM_VERSION@\nLicense: MIT\n")
	if err := os.MkdirAll(filepath.Join(dir, ".tito"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Specs[0].Method != "make_srpm" {
		t.Errorf("method = %q, want make_srpm for an unrenderable template", res.Specs[0].Method)
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a warning that tito was overridden")
	}
}

func TestDetectReleasePlaceholderIsUncertain(t *testing.T) {
	dir := t.TempDir()
	writeSpecTemplate(t, dir, "aetherpak.spec.in",
		"Name: aetherpak\nVersion: 1.0\nRelease: @RPM_RELEASE@\nLicense: MIT\n")
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Specs[0].Uncertain {
		t.Error("expected uncertain on release placeholder")
	}
	if res.Specs[0].Release != "@RPM_RELEASE@" {
		t.Errorf("release = %q, want the placeholder", res.Specs[0].Release)
	}
}

func TestDetectDualSpecWarns(t *testing.T) {
	dir := t.TempDir()
	writeSpec(t, dir, "Name: hello\nVersion: 1.0\nLicense: MIT\n")
	if err := os.WriteFile(filepath.Join(dir, "aetherpak.spec.in"),
		[]byte("Name: hello\nVersion: @RPM_VERSION@\nLicense: MIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Specs) != 2 {
		t.Fatalf("expected both specs, got %d", len(res.Specs))
	}
	if len(res.Warnings) == 0 {
		t.Error("expected a dual-spec warning")
	}
}

func TestDetectMakeTargetDecisionHasFlag(t *testing.T) {
	dir := t.TempDir()
	writeSpecTemplate(t, dir, "aetherpak.spec.in", "Name: aetherpak\nVersion: @RPM_VERSION@\nLicense: MIT\n")
	res, err := Detect(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range res.Decisions {
		if d.Field == "spec.makeSrpmTarget" {
			if d.Flag == "" || len(d.Proposal) == 0 {
				t.Errorf("decision needs a flag and proposal, got %+v", d)
			}
			return
		}
	}
	t.Error("expected makeSrpmTarget decision")
}
