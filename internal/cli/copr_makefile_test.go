package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/detect"
)

func TestRenderCoprMakefileRust(t *testing.T) {
	out := renderCoprMakefile(coprMakefileParams{
		SpecTemplate: "packaging/rpm/aetherpak.spec.in",
		SpecName:     "aetherpak.spec",
		PackageName:  "aetherpak",
		SourceName:   "aetherpak",
		HasCargo:     true,
		RustVersion:  "1.95",
		Subst: []tokenSubst{
			{Token: "@RPM_VERSION@", Expr: "$${rpm_version}"},
			{Token: "@RPM_RELEASE@", Expr: "$${rpm_release}"},
		},
	})
	for _, want := range []string{
		"SPEC_TEMPLATE := packaging/rpm/aetherpak.spec.in",
		"srpm:",
		"$(outdir)",
		"cargo vendor",
		".cargo/config.toml",
		"rpmbuild",
		"s|@RPM_VERSION@|$${rpm_version}|g",
		"$$work/SOURCES",
		"$$work/SPECS",
		"re-evaluates %{?dist} per chroot",
		"pre=\"$${rpm_version#*-}\"; rpm_version=\"$${rpm_version%%-*}\"",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("makefile missing %q", want)
		}
	}
	if !strings.Contains(out, "\n\tset -eu;") {
		t.Error("recipe lines must start with a tab")
	}
	if !strings.Contains(out, "REPO ?= $(shell git rev-parse --show-toplevel") {
		t.Error("repo-relative paths must anchor at the checkout top")
	}
	if !strings.Contains(out, "\"$(REPO)/$(SPEC_TEMPLATE)\"") {
		t.Error("template input must resolve through REPO")
	}
}

func TestRenderCoprMakefileNoCargo(t *testing.T) {
	out := renderCoprMakefile(coprMakefileParams{
		SpecTemplate: "rpm/plain.spec.in",
		SpecName:     "plain.spec",
		PackageName:  "plain",
		SourceName:   "plain",
	})
	if strings.Contains(out, "cargo vendor") {
		t.Error("non-Rust makefile must not vendor")
	}
	if !strings.Contains(out, "cp \"$(REPO)/$(SPEC_TEMPLATE)\" \"$$work/SPECS/$(SPEC_NAME)\"") {
		t.Error("template without placeholders should be copied, not sedded")
	}
}

func TestRenderCoprMakefileUnknownToken(t *testing.T) {
	out := renderCoprMakefile(coprMakefileParams{
		SpecTemplate: "rpm/plain.spec.in",
		SpecName:     "plain.spec",
		PackageName:  "plain",
		SourceName:   "plain",
		Subst:        []tokenSubst{{Token: "@CHANGELOG_DATE@", Expr: "$(CHANGELOG_DATE)", MakeVar: "CHANGELOG_DATE"}},
	})
	if !strings.Contains(out, "s|@CHANGELOG_DATE@|$(CHANGELOG_DATE)|g") {
		t.Error("unknown token should become a make variable")
	}
	if !strings.Contains(out, "pass CHANGELOG_DATE=... to make") {
		t.Error("unknown token should document the make knob")
	}
}

func TestRenderCoprMakefileNoRustVersion(t *testing.T) {
	out := renderCoprMakefile(coprMakefileParams{
		SpecTemplate: "rpm/plain.spec.in",
		SpecName:     "plain.spec",
		PackageName:  "plain",
		SourceName:   "plain",
		HasCargo:     true,
	})
	if strings.Contains(out, "detected as :") {
		t.Error("empty rust version must not render a dangling comment")
	}
}

func TestTemplateSubst(t *testing.T) {
	got := templateSubst("Version: @RPM_VERSION@\nRelease: @RPM_RELEASE@\nDate: @DATE@\nAgain: @DATE@\n")
	if len(got) != 3 {
		t.Fatalf("got %d substs, want 3 (deduped): %+v", len(got), got)
	}
	if got[0].Expr != "$${rpm_version}" || got[1].Expr != "$${rpm_release}" {
		t.Errorf("known tokens wrong: %+v", got[:2])
	}
	if got[2].Expr != "$(DATE)" || got[2].MakeVar != "DATE" {
		t.Errorf("unknown token wrong: %+v", got[2])
	}
}

func TestSourceStem(t *testing.T) {
	tests := []struct {
		text, fallback, want string
	}{
		{"%autosetup -n nono-%{version}\n", "nono-cli", "nono"},
		{"%autosetup -q -n foo-1.0\n", "foo", "foo-1.0"},
		{"Source0: foo-%{version}.tar.gz\n", "foo", "foo"},
		{"Source0: https://example.com/foo-2.0.tgz\n", "foo", "foo"},
		{"Source0: https://example.com/my-pkg-rc1.tgz\n", "foo", "my-pkg-rc1"},
		{"Name: foo\n", "foo", "foo"},
	}
	for _, tt := range tests {
		if got := sourceStem(tt.text, tt.fallback); got != tt.want {
			t.Errorf("sourceStem(%q) = %q, want %q", tt.text, got, tt.want)
		}
	}
}

func TestMakefileParamsSkipsExistingMake(t *testing.T) {
	res := &detect.Result{HasCoprMake: true, Specs: []detect.SpecInfo{{Path: "a.spec.in", Template: true}}}
	if _, ok := makefileParams(res); ok {
		t.Error("expected no scaffold when .copr/Makefile exists")
	}
}

func TestMakefileParamsFromTemplate(t *testing.T) {
	dir := t.TempDir()
	specDir := filepath.Join(dir, "packaging", "rpm")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	template := "Name: aetherpak\nVersion: @RPM_VERSION@\nRelease: @RPM_RELEASE@\n" +
		"Source0: widget-%{version}.tar.gz\n%autosetup -n widget-%{version}\n"
	if err := os.WriteFile(filepath.Join(specDir, "aetherpak.spec.in"), []byte(template), 0o644); err != nil {
		t.Fatal(err)
	}
	res := &detect.Result{
		RepoDir:     dir,
		HasCargo:    true,
		RustVersion: "1.95",
		Specs:       []detect.SpecInfo{{Path: "packaging/rpm/aetherpak.spec.in", Name: "aetherpak", Template: true}},
	}
	mp, ok := makefileParams(res)
	if !ok {
		t.Fatal("expected scaffold params for template spec")
	}
	if mp.SpecName != "aetherpak.spec" || !mp.HasCargo || mp.RustVersion != "1.95" {
		t.Errorf("params wrong: %+v", mp)
	}
	if mp.SourceName != "widget" {
		t.Errorf("source stem = %q, want widget parsed from %%autosetup", mp.SourceName)
	}
	if len(mp.Subst) != 2 {
		t.Errorf("subst = %+v, want the two version tokens", mp.Subst)
	}
}
