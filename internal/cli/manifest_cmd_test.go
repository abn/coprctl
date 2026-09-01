package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/manifest"
)

func TestApplyManifestCreateWiresSettings(t *testing.T) {
	var mu sync.Mutex
	projectGets := 0
	var createBody, setBody map[string]any
	setCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api_3/project/":
			projectGets++
			if projectGets == 1 {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"chroot_repos": map[string]string{"fedora-42-x86_64": "u"},
			})
		case "/api_3/project/add/quadzero":
			json.NewDecoder(r.Body).Decode(&createBody)
			w.WriteHeader(http.StatusOK)
		case "/api_3/project/permissions/set/quadzero/aetherpak":
			setCalled = true
			json.NewDecoder(r.Body).Decode(&setBody)
			json.NewEncoder(w).Encode(map[string]any{"updated": []string{"bob", "alice"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)

	m, err := manifest.Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  description: desc
  instructions: install me
  settings:
    enableNet: true
    develMode: true
    unlistedOnHomepage: true
  permissions:
    builders: [bob]
    admins: [alice]
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(context.Background(), app, m, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if createBody["instructions"] != "install me" {
		t.Errorf("create instructions = %v", createBody["instructions"])
	}
	if createBody["enable_net"] != true {
		t.Errorf("create enable_net = %v", createBody["enable_net"])
	}
	if createBody["unlisted_on_hp"] != true {
		t.Errorf("create unlisted_on_hp = %v", createBody["unlisted_on_hp"])
	}
	if createBody["devel_mode"] != true {
		t.Errorf("create devel_mode = %v", createBody["devel_mode"])
	}
	if !setCalled {
		t.Fatal("permissions set not called")
	}
	b, ok := setBody["bob"].(map[string]any)
	if !ok || b["builder"] != "approved" {
		t.Errorf("bob = %v", setBody["bob"])
	}
	if _, ok := b["admin"]; ok {
		t.Errorf("bob admin should be omitted: %v", b)
	}
	a, ok := setBody["alice"].(map[string]any)
	if !ok || a["admin"] != "approved" {
		t.Errorf("alice = %v", setBody["alice"])
	}
}

func TestApplyManifestEditWiresSettings(t *testing.T) {
	var mu sync.Mutex
	var editBody, setBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"chroot_repos": map[string]string{"fedora-42-x86_64": "u"},
			})
		case "/api_3/project/edit/quadzero/aetherpak":
			json.NewDecoder(r.Body).Decode(&editBody)
			w.WriteHeader(http.StatusOK)
		case "/api_3/project/permissions/set/quadzero/aetherpak":
			json.NewDecoder(r.Body).Decode(&setBody)
			json.NewEncoder(w).Encode(map[string]any{"updated": []string{"bob"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)

	m, err := manifest.Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  description: desc
  instructions: install me
  settings:
    enableNet: false
  permissions:
    builders: [bob]
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(context.Background(), app, m, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if editBody["instructions"] != "install me" {
		t.Errorf("edit instructions = %v", editBody["instructions"])
	}
	if editBody["enable_net"] != false {
		t.Errorf("edit enable_net = %v", editBody["enable_net"])
	}
	b, ok := setBody["bob"].(map[string]any)
	if !ok || b["builder"] != "approved" {
		t.Errorf("bob = %v", setBody["bob"])
	}
}

func TestApplyManifestCreatePackageSendsDeclaredAutoRebuild(t *testing.T) {
	var mu sync.Mutex
	var addBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"chroot_repos": map[string]string{},
			})
		case r.URL.Path == "/api_3/project/edit/quadzero/aetherpak":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/api_3/package/add/"):
			json.NewDecoder(r.Body).Decode(&addBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"name": "hello"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)

	m, err := manifest.Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  packages:
    - name: hello
      source: {type: scm, cloneUrl: https://github.com/abn/hello-rpm}
      autoRebuild: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(context.Background(), app, m, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if addBody["webhook_rebuild"] != true {
		t.Errorf("declared autoRebuild must send webhook_rebuild on create, got %v", addBody["webhook_rebuild"])
	}
}

func TestValidateCmdWarningsExitZero(t *testing.T) {
	app := NewApp()
	cmd := newValidateCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	dir := t.TempDir()
	fp := filepath.Join(dir, "copr.yaml")
	if err := os.WriteFile(fp, []byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: o, name: n}
spec:
  settings:
    persistent: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"-f", fp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate with only warnings should exit 0, got %v", err)
	}
	if !strings.Contains(buf.String(), "warning spec.settings.persistent") {
		t.Errorf("output = %q, want warning rendered", buf.String())
	}
	if !strings.Contains(buf.String(), "manifest valid") {
		t.Errorf("output = %q, want manifest valid", buf.String())
	}
}

func TestApplyManifestCreateWiresDeclaredSettings(t *testing.T) {
	var mu sync.Mutex
	projectGets := 0
	var createBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api_3/project/":
			projectGets++
			if projectGets == 1 {
				http.NotFound(w, r)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"chroot_repos": map[string]string{},
			})
		case "/api_3/project/add/quadzero":
			json.NewDecoder(r.Body).Decode(&createBody)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)

	m, err := manifest.Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  settings:
    persistent: true
    storage: pulp
    autoPrune: true
    bootstrap: on
    isolation: nspawn
    moduleHotfixes: true
    appstream: true
    packitForgeProjectsAllowed: [github.com/quadzero/aetherpak]
    followFedoraBranching: true
    repoPriority: 50
    multilib: true
    fedoraReview: true
    runtimeDependencies: [https://repo.example.com/fedora/]
    deleteAfterDays: 7
    unlistedOnHomepage: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(context.Background(), app, m, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := map[string]any{
		"persistent":                    true,
		"storage":                       "pulp",
		"auto_prune":                    true,
		"bootstrap":                     "on",
		"isolation":                     "nspawn",
		"module_hotfixes":               true,
		"appstream":                     true,
		"packit_forge_projects_allowed": []any{"github.com/quadzero/aetherpak"},
		"follow_fedora_branching":       true,
		"repo_priority":                 float64(50),
		"multilib":                      true,
		"fedora_review":                 true,
		"runtime_dependencies":          []any{"https://repo.example.com/fedora/"},
		"delete_after_days":             float64(7),
		"unlisted_on_hp":                true,
	}
	for k, wantV := range want {
		if gotV, ok := createBody[k]; !ok || fmt.Sprint(gotV) != fmt.Sprint(wantV) {
			t.Errorf("create body[%q] = %v (%v), want %v", k, gotV, ok, wantV)
		}
	}
}

func TestApplyManifestEditWiresDeclaredSettings(t *testing.T) {
	var mu sync.Mutex
	var editBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"chroot_repos": map[string]string{},
			})
		case "/api_3/project/edit/quadzero/aetherpak":
			json.NewDecoder(r.Body).Decode(&editBody)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)

	m, err := manifest.Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  settings:
    autoPrune: true
    bootstrap: on
    moduleHotfixes: true
    repoPriority: 50
    deleteAfterDays: 30
    unlistedOnHomepage: true
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(context.Background(), app, m, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	want := map[string]any{
		"auto_prune":        true,
		"bootstrap":         "on",
		"module_hotfixes":   true,
		"repo_priority":     float64(50),
		"delete_after_days": float64(30),
		"unlisted_on_hp":    true,
	}
	for k, wantV := range want {
		if gotV, ok := editBody[k]; !ok || fmt.Sprint(gotV) != fmt.Sprint(wantV) {
			t.Errorf("edit body[%q] = %v (%v), want %v", k, gotV, ok, wantV)
		}
	}
	// Declared-only: settings the manifest does not mention stay absent, and
	// the create-only fields are never sent on edit.
	for _, k := range []string{"persistent", "storage", "isolation", "appstream",
		"packit_forge_projects_allowed", "follow_fedora_branching", "multilib",
		"fedora_review", "runtime_dependencies"} {
		if _, ok := editBody[k]; ok {
			t.Errorf("undeclared field %q sent on edit: %v", k, editBody[k])
		}
	}
}

func TestApplyManifestExistingPackageEditFallback(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	var editBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"chroot_repos": map[string]string{},
			})
		case r.URL.Path == "/api_3/project/edit/quadzero/aetherpak":
			w.WriteHeader(http.StatusOK)
		case strings.HasPrefix(r.URL.Path, "/api_3/package/add/"):
			calls++
			http.Error(w, `{"error": "Package already exists in this project."}`, http.StatusBadRequest)
		case strings.HasPrefix(r.URL.Path, "/api_3/package/edit/"):
			json.NewDecoder(r.Body).Decode(&editBody)
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{"name": "hello"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)

	m, err := manifest.Parse([]byte(`apiVersion: coprctl/v1
kind: Project
metadata: {owner: quadzero, name: aetherpak}
spec:
  packages:
    - name: hello
      source: {type: scm, cloneUrl: https://github.com/abn/hello-rpm}
      maxBuilds: 5
      timeout: 3600
      chrootDenylist: [fedora-rawhide-*]
`))
	if err != nil {
		t.Fatal(err)
	}
	if err := applyManifest(context.Background(), app, m, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if calls != 1 {
		t.Errorf("add calls = %d, want 1 (conflict before the edit fallback)", calls)
	}
	if editBody["max_builds"] != float64(5) {
		t.Errorf("edit max_builds = %v", editBody["max_builds"])
	}
	if editBody["timeout"] != float64(3600) {
		t.Errorf("edit timeout = %v", editBody["timeout"])
	}
	if editBody["chroot_denylist"] != "fedora-rawhide-*" {
		t.Errorf("edit chroot_denylist = %v", editBody["chroot_denylist"])
	}
	if editBody["clone_url"] != "https://github.com/abn/hello-rpm" {
		t.Errorf("edit clone_url = %v, existing source must be preserved", editBody["clone_url"])
	}
	if _, ok := editBody["webhook_rebuild"]; ok {
		t.Errorf("undeclared autoRebuild must not send webhook_rebuild on edit: %v", editBody["webhook_rebuild"])
	}
}
