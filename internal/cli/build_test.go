package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestFailedChroots(t *testing.T) {
	tests := []struct {
		name  string
		build *copr.Build
		want  []string
	}{
		{
			name: "detailed builds map",
			build: &copr.Build{
				Builds: map[string]*copr.BuildChroot{
					"fedora-rawhide-x86_64": {State: "failed"},
					"fedora-41-x86_64":      {State: "succeeded"},
					"epel-9-x86_64":         {State: "failed"},
					"fedora-42-x86_64":      {State: "running"},
				},
			},
			want: []string{"epel-9-x86_64", "fedora-rawhide-x86_64"},
		},
		{
			name: "chroots list fallback",
			build: &copr.Build{
				Chroots: []string{"fedora-rawhide-x86_64", "epel-9-x86_64"},
				State:   "failed",
			},
			want: []string{"epel-9-x86_64", "fedora-rawhide-x86_64"},
		},
		{
			name: "no failed chroots",
			build: &copr.Build{
				Builds: map[string]*copr.BuildChroot{
					"fedora-rawhide-x86_64": {State: "succeeded"},
					"epel-9-x86_64":         {State: "skipped"},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := failedChroots(tt.build)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("failedChroots = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRebuildOnlyFailed(t *testing.T) {
	listRaw, err := os.ReadFile("../copr/testdata/build-chroot-list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/build/42":
			json.NewEncoder(w).Encode(map[string]any{
				"id":              42,
				"state":           "failed",
				"ownername":       "owner",
				"projectname":     "proj",
				"project_dirname": "proj",
				"chroots":         []string{},
				"source_package":  map[string]any{"name": "pkg", "version": "1.0", "url": nil},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/build-chroot/list":
			if r.URL.Query().Get("build_id") != "42" {
				t.Errorf("build_id query = %q, want 42", r.URL.Query().Get("build_id"))
			}
			w.Write(listRaw)
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": 1, "name": "pkg", "source_type": "scm", "source_dict": map[string]any{"type": "git", "clone_url": "https://example.com/r.git"}},
				},
				"meta": copr.Meta{Limit: 100, Offset: 0, Order: "id", OrderType: "ASC"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api_3/build/create/scm":
			gotBody, _ = io.ReadAll(r.Body)
			json.NewEncoder(w).Encode(map[string]any{"id": 999, "state": "pending"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"rebuild", "owner/proj/pkg", "--only-failed", "42", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("decode submitted body: %v", err)
	}
	chroots, _ := sent["chroot_names"].([]any)
	if len(chroots) != 1 || chroots[0] != "epel-9-x86_64" {
		t.Errorf("chroot_names = %v, want exactly the failed chroot epel-9-x86_64", sent["chroot_names"])
	}
}

func TestBuildRebuildOnlyFailedDegraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/build/42":
			json.NewEncoder(w).Encode(map[string]any{
				"id":              42,
				"state":           "failed",
				"ownername":       "owner",
				"projectname":     "proj",
				"project_dirname": "proj",
				"chroots":         []string{},
				"source_package":  map[string]any{"name": "pkg", "version": "1.0", "url": nil},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/build-chroot/list":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"rebuild", "owner/proj/pkg", "--only-failed", "42", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error when per-chroot detail cannot be fetched")
	}
	if !strings.Contains(err.Error(), "not submitted") {
		t.Errorf("error = %q, want a message noting the rebuild was not submitted", err)
	}
}

func TestBuildSubmitUploadRoutesToUploadBuild(t *testing.T) {
	var path, ct string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		ct = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), `name="json"`) || !strings.Contains(string(body), `name="pkgs"`) {
			t.Errorf("multipart body missing json/pkgs parts")
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": 99, "state": "pending"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	srpm := filepath.Join(dir, "x.src.rpm")
	if err := os.WriteFile(srpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "upload", "--upload", srpm, "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if path != "/api_3/build/create/upload" {
		t.Errorf("path = %q, want the multipart upload endpoint", path)
	}
	if !strings.Contains(ct, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", ct)
	}
}
