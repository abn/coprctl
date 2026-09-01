package manifest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
	// The 9 readable settings are exported; create-only and write-only fields
	// are not, since apply cannot reconcile them on an existing project.
	if !m.Spec.Settings.AutoPrune {
		t.Errorf("autoPrune = %v, want live default true", m.Spec.Settings.AutoPrune)
	}
	if m.Spec.Settings.Bootstrap != "default" || m.Spec.Settings.Isolation != "default" {
		t.Errorf("bootstrap/isolation = %q/%q", m.Spec.Settings.Bootstrap, m.Spec.Settings.Isolation)
	}
	if m.Spec.Settings.ModuleHotfixes || m.Spec.Settings.Appstream {
		t.Errorf("moduleHotfixes/appstream = %v/%v", m.Spec.Settings.ModuleHotfixes, m.Spec.Settings.Appstream)
	}
	if len(m.Spec.Settings.PackitForgeProjectsAllowed) != 0 {
		t.Errorf("packitForgeProjectsAllowed = %v", m.Spec.Settings.PackitForgeProjectsAllowed)
	}
	if !m.Spec.Settings.FollowFedoraBranching {
		t.Errorf("followFedoraBranching = %v, want live default true", m.Spec.Settings.FollowFedoraBranching)
	}
	if m.Spec.Settings.RepoPriority != 99 {
		t.Errorf("repoPriority = %d, want live default 99", m.Spec.Settings.RepoPriority)
	}
	if m.Spec.Settings.UnlistedOnHomepage {
		t.Errorf("unlistedOnHomepage = %v, want false", m.Spec.Settings.UnlistedOnHomepage)
	}
	if m.Spec.Settings.Persistent || m.Spec.Settings.Storage != "" {
		t.Errorf("persistent/storage = %v/%q, must not export create-only fields", m.Spec.Settings.Persistent, m.Spec.Settings.Storage)
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
    runtimeDependencies: [https://repo.example.com/fedora/]
    packitForgeProjectsAllowed: [github.com/quadzero/aetherpak]
    unlistedOnHomepage: true
    additionalRepos: [repo]
    persistent: true
    storage: pulp
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	// Only the create-only settings warn now; the rest are applied on both
	// create and edit, and unlisted_on_hp is editable (H3).
	warnPaths := map[string]bool{
		"spec.settings.additionalRepos": true,
		"spec.settings.persistent":      true,
		"spec.settings.storage":         true,
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

func TestDiffAgainstDeclaredSettingsDrift(t *testing.T) {
	// Live values are chosen so every declared setting can drift: bools
	// default false (a manifest can only express true), bootstrap/isolation
	// default "default", repo_priority defaults to 99, and the forge list is
	// empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/project/" && r.URL.Path != "/api_3/package/list" &&
			r.URL.Path != "/api_3/project/permissions/get/quadzero/aetherpak" {
			http.NotFound(w, r)
			return
		}
		switch r.URL.Path {
		case "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"persistent":                    false,
				"auto_prune":                    false,
				"bootstrap":                     "default",
				"isolation":                     "default",
				"module_hotfixes":               false,
				"appstream":                     false,
				"packit_forge_projects_allowed": []string{},
				"follow_fedora_branching":       false,
				"repo_priority":                 99,
				"storage":                       "backend",
				"unlisted_on_hp":                false,
				"chroot_repos":                  map[string]string{},
			})
		case "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{"items": []map[string]any{}})
		case "/api_3/project/permissions/get/quadzero/aetherpak":
			json.NewEncoder(w).Encode(map[string]any{"permissions": map[string]any{}})
		}
	}))
	defer srv.Close()
	c := copr.New(srv.URL, nil)
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  settings:
    autoPrune: true
    bootstrap: on
    isolation: nspawn
    moduleHotfixes: true
    appstream: true
    packitForgeProjectsAllowed: [github.com/quadzero/aetherpak]
    followFedoraBranching: true
    repoPriority: 42
    unlistedOnHomepage: true
    persistent: true
    storage: pulp
`))
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := m.DiffAgainst(context.Background(), c)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	wantPaths := map[string]bool{
		"spec.settings.autoPrune":                  true,
		"spec.settings.bootstrap":                  true,
		"spec.settings.isolation":                  true,
		"spec.settings.moduleHotfixes":             true,
		"spec.settings.appstream":                  true,
		"spec.settings.packitForgeProjectsAllowed": true,
		"spec.settings.followFedoraBranching":      true,
		"spec.settings.repoPriority":               true,
		"spec.settings.unlistedOnHomepage":         true,
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
	// persistent and storage are create-only and never diffed: the edit API
	// has no field for them, so apply could never converge the drift.
	for p := range got {
		if p == "spec.settings.persistent" || p == "spec.settings.storage" {
			t.Errorf("create-only field %q must not diff, got %+v", p, diffs)
		}
	}
}

func TestDiffAgainstMinimalManifestClean(t *testing.T) {
	// H1: a manifest that declares no settings must not drift against the live
	// defaults (auto_prune true, bootstrap "default", isolation "default",
	// follow_fedora_branching true), because apply is declared-only and could
	// never fix an unconditional comparison.
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
		if strings.HasPrefix(d.Path, "spec.settings") {
			t.Errorf("minimal manifest drifted on undeclared setting %q: %+v", d.Path, d)
		}
	}
}

func TestValidatePersistentWithDeleteAfterDays(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  settings:
    persistent: true
    deleteAfterDays: 30
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	for _, i := range issues {
		if i.Level == "error" && strings.Contains(i.Detail, "persistent") {
			return
		}
	}
	t.Fatalf("expected persistent+deleteAfterDays error, got %+v", issues)
}

func TestValidatePackageSettings(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  packages:
    - name: ok
      source: {type: scm, cloneUrl: https://example.com/r.git}
      maxBuilds: 100
      timeout: 0
      chrootDenylist: [fedora-rawhide-*, 'epel-9-*']
    - name: bad-max
      source: {type: scm, cloneUrl: https://example.com/r.git}
      maxBuilds: 101
    - name: bad-timeout
      source: {type: scm, cloneUrl: https://example.com/r.git}
      timeout: -1
    - name: bad-timeout-upper
      source: {type: scm, cloneUrl: https://example.com/r.git}
      timeout: 108001
    - name: bad-pattern
      source: {type: scm, cloneUrl: https://example.com/r.git}
      chrootDenylist: ['fedora rawhide']
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	want := map[string]bool{
		"spec.packages[1].maxBuilds":      true,
		"spec.packages[2].timeout":        true,
		"spec.packages[3].timeout":        true,
		"spec.packages[4].chrootDenylist": true,
	}
	for _, i := range issues {
		if i.Level == "error" {
			delete(want, i.Path)
		}
	}
	if len(want) > 0 {
		t.Errorf("missing validation errors for: %v (got %+v)", want, issues)
	}
}

func TestValidateDeleteAfterDaysRange(t *testing.T) {
	m, err := Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  settings:
    deleteAfterDays: 721
`))
	if err != nil {
		t.Fatal(err)
	}
	issues := m.Validate()
	for _, i := range issues {
		if i.Level == "error" && i.Path == "spec.settings.deleteAfterDays" {
			m, err = Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  settings:
    deleteAfterDays: -2
`))
			if err != nil {
				t.Fatal(err)
			}
			for _, i := range m.Validate() {
				if i.Level == "error" && i.Path == "spec.settings.deleteAfterDays" {
					return
				}
			}
			t.Fatalf("expected deleteAfterDays range error for -2, got %+v", m.Validate())
		}
	}
	t.Fatalf("expected deleteAfterDays range error for 721, got %+v", issues)
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
