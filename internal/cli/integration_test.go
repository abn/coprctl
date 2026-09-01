package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/config"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/state"
)

func TestNormalizeCloneURLForgeAgnostic(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/abn/coprctl.git", "github.com/abn/coprctl"},
		{"https://github.com/abn/coprctl", "github.com/abn/coprctl"},
		{"git@github.com:abn/coprctl.git", "github.com/abn/coprctl"},
		{"http://github.com/abn/coprctl", "github.com/abn/coprctl"},
		{"https://gitlab.com/group/repo.git", "gitlab.com/group/repo"},
		{"git@gitlab.com:group/repo.git", "gitlab.com/group/repo"},
		{"ssh://git@gitlab.example.com:2222/group/repo.git", "gitlab.example.com/group/repo"},
	}
	for _, tc := range cases {
		if got := normalizeCloneURL(tc.in); got != tc.want {
			t.Errorf("normalizeCloneURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMaskSecretCustomForge(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://copr.test/webhooks/github/42/s3cr3t/", "https://copr.test/webhooks/github/42/****/"},
		{"https://copr.test/webhooks/gitlab/42/s3cr3t/pkg/", "https://copr.test/webhooks/gitlab/42/****/pkg/"},
		{"https://copr.test/webhooks/bitbucket/42/s3cr3t/", "https://copr.test/webhooks/bitbucket/42/****/"},
		{"https://copr.test/webhooks/custom/42/s3cr3t/pkg/", "https://copr.test/webhooks/custom/42/****/pkg/"},
		{"https://copr.test/not-a-webhook", "https://copr.test/not-a-webhook"},
	}
	for _, tc := range cases {
		if got := maskSecret(tc.in); got != tc.want {
			t.Errorf("maskSecret(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// integrationCoprServer serves the Copr endpoints the integration commands use:
// project lookup, package listing, and package edits for auto-rebuild.
func integrationCoprServer(t *testing.T, cloneURL string, edits *int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{"id": 42, "name": "p", "ownername": "o", "full_name": "o/p"})
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{
					"id": 1, "name": "pkg", "source_type": "scm", "auto_rebuild": false,
					"source_dict": map[string]any{"type": "git", "clone_url": cloneURL},
				}},
				"meta": copr.Meta{Limit: 100, Offset: 0, Order: "id", OrderType: "ASC"},
			})
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/api_3/package/edit/"):
			if edits != nil {
				*edits++
			}
			w.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected copr request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

// integrationTestApp builds an App bound to a fake Copr server and a fixed
// instance base URL, so webhook URLs are deterministic.
func integrationTestApp(t *testing.T, coprSrv *httptest.Server) *App {
	t.Helper()
	cfgDir := t.TempDir()
	cfgPath := filepath.Join(cfgDir, "config.toml")
	m := config.New(cfgPath, filepath.Join(cfgDir, "legacy"))
	if err := m.SetProfile("test", config.Profile{URL: "https://copr.test"}); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	app.Cfg = m
	app.cfgPath = cfgPath
	app.profile = "test"
	app.client = copr.New(coprSrv.URL, nil)
	return app
}

// integrationTestState points the per-profile state store at a temp dir.
func integrationTestState(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

// seedIntegrationState records a webhook secret and optionally a hook id for
// the o/p project used by the fake servers.
func seedIntegrationState(t *testing.T, secret string, hookID int64) {
	t.Helper()
	store, err := state.NewStore(mustStateDir("test"))
	if err != nil {
		t.Fatal(err)
	}
	if secret != "" {
		if err := store.SetSecret("o", "p", secret); err != nil {
			t.Fatal(err)
		}
	}
	if hookID != 0 {
		if err := store.SetHookID("o", "p", hookID); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWebhookURLForgeSegments(t *testing.T) {
	srv := integrationCoprServer(t, "", nil)
	defer srv.Close()
	integrationTestState(t)
	seedIntegrationState(t, "s3cr3t", 0)
	app := integrationTestApp(t, srv)
	ctx := context.Background()
	r := ref.Ref{Owner: "o", Project: "p"}

	tests := []struct{ name, forge, pkg, want string }{
		{"github", "github", "", "https://copr.test/webhooks/github/42/s3cr3t/"},
		{"github package-scoped", "github", "pkg", "https://copr.test/webhooks/github/42/s3cr3t/pkg/"},
		{"gitlab", "gitlab", "", "https://copr.test/webhooks/gitlab/42/s3cr3t/"},
		{"gitlab package-scoped", "gitlab", "pkg", "https://copr.test/webhooks/gitlab/42/s3cr3t/pkg/"},
		{"custom package-scoped", "custom", "pkg", "https://copr.test/webhooks/custom/42/s3cr3t/pkg/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := webhookURL(ctx, app, r, tt.forge, tt.pkg)
			if err != nil {
				t.Fatalf("webhookURL: %v", err)
			}
			if got != tt.want {
				t.Errorf("webhookURL(%s,%q) = %q, want %q", tt.forge, tt.pkg, got, tt.want)
			}
		})
	}

	if _, err := webhookURL(ctx, app, r, "custom", ""); err == nil {
		t.Error("custom without a package must fail")
	}
	if _, err := webhookURL(ctx, app, r, "pagure", ""); err == nil {
		t.Error("unknown forge must fail")
	}
}

func TestIntegrationDisableVerifiesBeforeDelete(t *testing.T) {
	expected := "https://copr.test/webhooks/gitlab/42/s3cr3t/pkg/"
	var deleted []string
	glSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/projects/o%2Fr/hooks":
			json.NewEncoder(w).Encode([]map[string]any{
				{"id": 7, "url": expected},
				// A stale hook aimed at another project must never be deleted.
				{"id": 999, "url": "https://copr.test/webhooks/gitlab/99/other/pkg/"},
			})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.EscapedPath(), "/projects/o%2Fr/hooks/"):
			deleted = append(deleted, r.URL.EscapedPath())
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected gitlab request %s %s", r.Method, r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	defer glSrv.Close()
	t.Setenv("GITLAB_TOKEN", "tok")
	t.Setenv("GITLAB_API_URL", glSrv.URL)

	coprSrv := integrationCoprServer(t, "https://gitlab.com/o/r.git", nil)
	defer coprSrv.Close()
	integrationTestState(t)
	// The stored id is stale and must not steer the delete.
	seedIntegrationState(t, "s3cr3t", 999)
	app := integrationTestApp(t, coprSrv)

	cmd := newIntegrationCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"disable", "o/p", "--forge", "gitlab", "--repo", "o/r", "--yes", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "/projects/o%2Fr/hooks/7" {
		t.Fatalf("deleted = %v, want exactly /projects/o%%2Fr/hooks/7", deleted)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if out["disabled"] != true || out["hook_id"] != float64(7) || out["stored_hook_id"] != float64(0) {
		t.Errorf("output = %v, want disabled hook_id=7 stored_hook_id=0", out)
	}
	if out["url"] != "https://copr.test/webhooks/gitlab/42/****/pkg/" {
		t.Errorf("url = %v, want masked url", out["url"])
	}
}

func TestIntegrationEnableReusesReceiverPrefix(t *testing.T) {
	var patched []string
	var updated map[string]any
	glSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/projects/o%2Fr/hooks":
			json.NewEncoder(w).Encode([]map[string]any{
				// Stale secret, same project id: must be reused and re-pointed.
				{"id": 7, "url": "https://copr.test/webhooks/gitlab/42/olds3cr3t/pkg/"},
				// Another project id: must never be touched.
				{"id": 99, "url": "https://copr.test/webhooks/gitlab/99/other/pkg/"},
			})
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.EscapedPath(), "/projects/o%2Fr/hooks/"):
			patched = append(patched, r.URL.EscapedPath())
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &updated); err != nil {
				t.Errorf("update body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/o%2Fr/hooks/7/test/tag_push_events":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"message": "201 Created"})
		default:
			t.Errorf("unexpected gitlab request %s %s", r.Method, r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	defer glSrv.Close()
	t.Setenv("GITLAB_TOKEN", "tok")
	t.Setenv("GITLAB_API_URL", glSrv.URL)

	editCalls := 0
	coprSrv := integrationCoprServer(t, "https://gitlab.com/o/r.git", &editCalls)
	defer coprSrv.Close()
	integrationTestState(t)
	seedIntegrationState(t, "s3cr3t", 0)
	app := integrationTestApp(t, coprSrv)

	cmd := newIntegrationCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"gitlab", "enable", "o/p", "--repo", "o/r", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(patched) != 1 || patched[0] != "/projects/o%2Fr/hooks/7" {
		t.Fatalf("patched = %v, want exactly /projects/o%%2Fr/hooks/7", patched)
	}
	if updated["url"] != "https://copr.test/webhooks/gitlab/42/s3cr3t/pkg/" {
		t.Errorf("update url = %v, want re-pointed at the current secret", updated["url"])
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if out["hook_id"] != float64(7) {
		t.Errorf("hook_id = %v, want 7 (reused, not created)", out["hook_id"])
	}
}

func TestIntegrationGitlabEnableWarnsOnTestFailure(t *testing.T) {
	glSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 7, "url": "https://copr.test/webhooks/gitlab/42/s3cr3t/pkg/"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks/7/test/tag_push_events":
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			t.Errorf("unexpected gitlab request %s %s", r.Method, r.URL.EscapedPath())
			http.NotFound(w, r)
		}
	}))
	defer glSrv.Close()
	t.Setenv("GITLAB_TOKEN", "tok")
	t.Setenv("GITLAB_API_URL", glSrv.URL)

	editCalls := 0
	coprSrv := integrationCoprServer(t, "https://gitlab.com/group/repo.git", &editCalls)
	defer coprSrv.Close()
	integrationTestState(t)
	seedIntegrationState(t, "s3cr3t", 0)
	app := integrationTestApp(t, coprSrv)

	cmd := newIntegrationCmd(app)
	var buf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"gitlab", "enable", "o/p", "--repo", "group/repo", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(errBuf.String(), "warning:") {
		t.Errorf("stderr = %q, want a warning for the failed test hook", errBuf.String())
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if out["enabled"] != true || out["hook_id"] != float64(7) {
		t.Errorf("output = %v, want enabled hook_id=7 despite test failure", out)
	}
}

func TestIntegrationGitlabEnableSendsBooleanToggles(t *testing.T) {
	var created map[string]any
	editCalls := 0
	glSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks":
			json.NewEncoder(w).Encode([]map[string]any{})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks":
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &created); err != nil {
				t.Errorf("create body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": 7, "url": "https://copr.test/webhooks/gitlab/42/s3cr3t/pkg/"})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks/7/test/tag_push_events":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"message": "201 Created"})
		default:
			t.Errorf("unexpected gitlab request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer glSrv.Close()
	t.Setenv("GITLAB_TOKEN", "tok")
	t.Setenv("GITLAB_API_URL", glSrv.URL)

	coprSrv := integrationCoprServer(t, "https://gitlab.com/group/repo.git", &editCalls)
	defer coprSrv.Close()
	integrationTestState(t)
	seedIntegrationState(t, "s3cr3t", 0)
	app := integrationTestApp(t, coprSrv)

	cmd := newIntegrationCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"gitlab", "enable", "o/p", "--repo", "group/repo", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	// Tag-only default maps to GitLab's boolean toggles, not an event list.
	if created["push_events"] != false {
		t.Errorf("push_events = %v, want false for tag-only", created["push_events"])
	}
	if created["tag_push_events"] != true {
		t.Errorf("tag_push_events = %v, want true", created["tag_push_events"])
	}
	if _, ok := created["events"]; ok {
		t.Error("gitlab hook payload must not carry an event-name list")
	}
	if editCalls != 1 {
		t.Errorf("package auto-rebuild edits = %d, want 1 (github path reused)", editCalls)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if out["enabled"] != true || out["hook_id"] != float64(7) || out["package"] != "pkg" {
		t.Errorf("output = %v, want enabled hook_id=7 package=pkg", out)
	}
	if out["url"] != "https://copr.test/webhooks/gitlab/42/****/pkg/" {
		t.Errorf("url = %v, want masked url", out["url"])
	}
}
