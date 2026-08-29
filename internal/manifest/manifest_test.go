package manifest

import (
	"context"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestParseAndValidate(t *testing.T) {
	data := []byte(`apiVersion: coprctl/v1
kind: Project
metadata:
  owner: quadzero
  name: aetherpak
spec:
  description: test
  chroots:
    enabled:
      - fedora-42-x86_64
  packages:
    - name: aetherpak
      source:
        type: scm
        cloneUrl: https://github.com/quadzero/aetherpak.git
`)
	m, err := Parse(data)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if m.Metadata.Name != "aetherpak" || len(m.Spec.Packages) != 1 {
		t.Fatalf("parsed manifest wrong: %+v", m)
	}
	if issues := m.Validate(); len(issues) != 0 {
		t.Fatalf("unexpected validation issues: %+v", issues)
	}
}

func TestValidateCatchesErrors(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  packages:
    - name: p
      source:
        type: scm
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	found := false
	for _, i := range issues {
		if i.Path == "spec.packages[0].source.cloneUrl" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected cloneUrl validation error, got %+v", issues)
	}
}

func TestParseMissingRequired(t *testing.T) {
	if _, err := Parse([]byte(`foo: bar`)); err == nil {
		t.Fatal("expected error for missing apiVersion/kind")
	}
}

func TestExportFromLive(t *testing.T) {
	srv := newManifestServer(t)
	c := copr.New(srv.URL, nil)
	m, err := ExportFromLive(context.Background(), c, "quadzero", "aetherpak")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if m.Spec.Description != "live desc" {
		t.Errorf("description = %q", m.Spec.Description)
	}
	if m.Spec.Instructions != "live instructions" {
		t.Errorf("instructions = %q", m.Spec.Instructions)
	}
	if m.Spec.Settings.EnableNet {
		t.Errorf("enableNet = %v, want false", m.Spec.Settings.EnableNet)
	}
	if len(m.Spec.Packages) != 1 || m.Spec.Packages[0].Name != "pkgo" {
		t.Errorf("packages = %+v", m.Spec.Packages)
	}
	wantPerms := Permissions{Builders: []string{"bob"}, Admins: []string{"alice"}}
	if !equalPermissions(m.Spec.Permissions, wantPerms) {
		t.Errorf("permissions = %+v, want %+v", m.Spec.Permissions, wantPerms)
	}
}

func TestDiffAgainstDrift(t *testing.T) {
	srv := newManifestServer(t)
	c := copr.New(srv.URL, nil)
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  description: other
  instructions: my instructions
  settings:
    enableNet: true
    develMode: true
  permissions:
    builders: [carol]
    admins: [alice]
`))
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := m.DiffAgainst(context.Background(), c)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	wantPaths := map[string]bool{
		"spec.description":          true,
		"spec.instructions":         true,
		"spec.settings.enableNet":   true,
		"spec.settings.develMode":   true,
		"spec.permissions.builders": true,
	}
	got := map[string]bool{}
	for _, d := range diffs {
		got[d.Path] = true
	}
	for p := range wantPaths {
		if !got[p] {
			t.Errorf("missing diff at %q, got %+v", p, diffs)
		}
	}
	if _, ok := got["spec.permissions.admins"]; ok {
		t.Errorf("admins should match live (alice approved), got %+v", diffs)
	}
}

func TestDiffAgainstPermissionsInSync(t *testing.T) {
	srv := newManifestServer(t)
	c := copr.New(srv.URL, nil)
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  permissions:
    builders: [bob]
    admins: [alice]
`))
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := m.DiffAgainst(context.Background(), c)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, d := range diffs {
		if d.Path == "spec.permissions.builders" || d.Path == "spec.permissions.admins" {
			t.Errorf("permissions should be in sync, got %+v", diffs)
		}
	}
}

func TestDiffAgainstPermissionsUndeclared(t *testing.T) {
	srv := newManifestServer(t)
	c := copr.New(srv.URL, nil)
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec: {}
`))
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := m.DiffAgainst(context.Background(), c)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	for _, d := range diffs {
		if d.Path == "spec.permissions.builders" || d.Path == "spec.permissions.admins" {
			t.Errorf("undeclared permissions must not diff, got %+v", diffs)
		}
	}
}

func TestValidateWarnsUnsupportedSettings(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  settings:
    appstream: true
    autoPrune: true
    followFedoraBranching: true
    moduleHotfixes: true
    multilib: true
    fedoraReview: true
    isolation: container
    bootstrap: on
    repoPriority: 50
    deleteAfterDays: 7
    additionalRepos: [repo]
    unlistedOnHomepage: true
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	warnPaths := map[string]bool{
		"spec.settings.appstream":             true,
		"spec.settings.autoPrune":             true,
		"spec.settings.followFedoraBranching": true,
		"spec.settings.moduleHotfixes":        true,
		"spec.settings.multilib":              true,
		"spec.settings.fedoraReview":          true,
		"spec.settings.isolation":             true,
		"spec.settings.bootstrap":             true,
		"spec.settings.repoPriority":          true,
		"spec.settings.deleteAfterDays":       true,
		"spec.settings.additionalRepos":       true,
		"spec.settings.unlistedOnHomepage":    true,
	}
	var errorsFound []string
	for _, i := range issues {
		if i.Level == "error" {
			errorsFound = append(errorsFound, i.Path)
		}
		if i.Level != "warning" {
			continue
		}
		delete(warnPaths, i.Path)
	}
	if len(errorsFound) > 0 {
		t.Errorf("unexpected errors: %v", errorsFound)
	}
	if len(warnPaths) > 0 {
		t.Errorf("missing warnings for: %v", warnPaths)
	}
}

func TestValidateNoWarningsForDefaults(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec: {}
`))
	if err != nil {
		t.Fatal(err)
	}
	for _, i := range m.Validate() {
		if i.Level == "warning" {
			t.Errorf("unexpected warning for empty settings: %+v", i)
		}
	}
}

func TestPermissionSetFromManifest(t *testing.T) {
	p := PermissionSetFromManifest(Permissions{
		Builders: []string{"bob", "both"},
		Admins:   []string{"alice", "both"},
	})
	if p["bob"].Builder != copr.PermissionApproved || p["bob"].Admin != "" {
		t.Errorf("bob = %+v", p["bob"])
	}
	if p["alice"].Admin != copr.PermissionApproved || p["alice"].Builder != "" {
		t.Errorf("alice = %+v", p["alice"])
	}
	if p["both"].Admin != copr.PermissionApproved || p["both"].Builder != copr.PermissionApproved {
		t.Errorf("both = %+v", p["both"])
	}
}

func equalPermissions(a, b Permissions) bool {
	if len(a.Builders) != len(b.Builders) || len(a.Admins) != len(b.Admins) {
		return false
	}
	for i := range a.Builders {
		if a.Builders[i] != b.Builders[i] {
			return false
		}
	}
	for i := range a.Admins {
		if a.Admins[i] != b.Admins[i] {
			return false
		}
	}
	return true
}
