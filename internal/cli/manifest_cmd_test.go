package cli

import (
	"bytes"
	"context"
	"encoding/json"
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
    appstream: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd.SetArgs([]string{"-f", fp})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("validate with only warnings should exit 0, got %v", err)
	}
	if !strings.Contains(buf.String(), "warning spec.settings.appstream") {
		t.Errorf("output = %q, want warning rendered", buf.String())
	}
	if !strings.Contains(buf.String(), "manifest valid") {
		t.Errorf("output = %q, want manifest valid", buf.String())
	}
}
