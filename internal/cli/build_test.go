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

func TestBuildSubmitURLMultiBuildRender(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/build/create/url" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "state": "pending"},
				{"id": 2, "state": "running"},
			},
			"meta": map[string]any{},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "url", "--url", "https://example.com/a.spec", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var builds []copr.Build
	if err := json.Unmarshal(buf.Bytes(), &builds); err != nil {
		t.Fatalf("expected a JSON array of builds, got %q: %v", buf.String(), err)
	}
	if len(builds) != 2 || builds[0].ID != 1 || builds[1].ID != 2 {
		t.Errorf("builds = %+v, want both envelope items", builds)
	}
}

func TestBuildSubmitURLMultiBuildJSONL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "state": "pending"},
				{"id": 2, "state": "pending"},
			},
			"meta": map[string]any{},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "url", "--url", "https://example.com/a.spec", "--output", "jsonl"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("jsonl lines = %q, want one object per build", buf.String())
	}
	var first, second copr.Build
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 = %q: %v", lines[0], err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 2 = %q: %v", lines[1], err)
	}
	if first.ID != 1 || second.ID != 2 {
		t.Errorf("builds = %d, %d, want 1, 2", first.ID, second.ID)
	}
}

func TestBuildListJSONL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/build/list" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("offset") == "2" {
			json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "meta": map[string]any{}})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "state": "succeeded"},
				{"id": 2, "state": "running"},
			},
			"meta": map[string]any{},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"list", "owner/proj", "--output", "jsonl"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("jsonl lines = %q, want one object per build", buf.String())
	}
	var first, second copr.Build
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 = %q: %v", lines[0], err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 2 = %q: %v", lines[1], err)
	}
	if first.ID != 1 || second.ID != 2 {
		t.Errorf("builds = %d, %d, want 1, 2", first.ID, second.ID)
	}
}

func TestBuildSubmitURLMultiBuildHumanTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "state": "pending"},
				{"id": 2, "state": "running"},
			},
			"meta": map[string]any{},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "url", "--url", "https://example.com/a.spec", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"ID", "STATE", "1", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q: %q", want, out)
		}
	}
}

func TestBuildRebuildKeepsRebuiltChrootsRow(t *testing.T) {
	listRaw, err := os.ReadFile("../copr/testdata/build-chroot-list.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
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
			w.Write(listRaw)
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": 1, "name": "pkg", "source_type": "scm", "source_dict": map[string]any{"type": "git", "clone_url": "https://example.com/r.git"}},
				},
				"meta": copr.Meta{Limit: 100, Offset: 0, Order: "id", OrderType: "ASC"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api_3/build/create/scm":
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
	cmd.SetArgs([]string{"rebuild", "owner/proj/pkg", "--only-failed", "42", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Rebuilt chroots") {
		t.Errorf("vertical rebuild table missing the Rebuilt chroots row: %q", out)
	}
	if !strings.Contains(out, "FIELD") || !strings.Contains(out, "VALUE") {
		t.Errorf("rebuild should keep the vertical FIELD/VALUE table, got %q", out)
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
