package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
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

func TestBuildSubmitOptionFlagsEndToEnd(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		check func(t *testing.T, body map[string]any)
	}{
		{
			name: "absent bools are not sent",
			args: []string{"--source", "scm", "--clone-url", "https://example.com/r.git"},
			check: func(t *testing.T, body map[string]any) {
				for _, key := range []string{"background", "enable_net", "timeout", "bootstrap", "isolation", "after_build_id", "with_build_id", "exclude_chroots"} {
					if _, ok := body[key]; ok {
						t.Errorf("%s = %v, want absent", key, body[key])
					}
				}
			},
		},
		{
			name: "set bools and options carry wire keys",
			args: []string{"--source", "scm", "--clone-url", "https://example.com/r.git",
				"--background", "--enable-net", "--timeout", "3600", "--bootstrap", "on",
				"--isolation", "simple", "--exclude-chroot", "fedora-rawhide-*"},
			check: func(t *testing.T, body map[string]any) {
				if body["background"] != true {
					t.Errorf("background = %v", body["background"])
				}
				if body["enable_net"] != true {
					t.Errorf("enable_net = %v", body["enable_net"])
				}
				if body["timeout"] != float64(3600) {
					t.Errorf("timeout = %v", body["timeout"])
				}
				if body["bootstrap"] != "on" {
					t.Errorf("bootstrap = %v", body["bootstrap"])
				}
				if body["isolation"] != "simple" {
					t.Errorf("isolation = %v", body["isolation"])
				}
				exc, ok := body["exclude_chroots"].([]any)
				if !ok || len(exc) != 1 || exc[0] != "fedora-rawhide-*" {
					t.Errorf("exclude_chroots = %v", body["exclude_chroots"])
				}
			},
		},
		{
			name: "explicit enable-net=false is sent",
			args: []string{"--source", "scm", "--clone-url", "https://example.com/r.git",
				"--enable-net=false"},
			check: func(t *testing.T, body map[string]any) {
				if body["enable_net"] != false {
					t.Errorf("enable_net = %v, want explicit false", body["enable_net"])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api_3/build/create/scm" {
					t.Errorf("path = %s", r.URL.Path)
				}
				gotBody, _ = io.ReadAll(r.Body)
				json.NewEncoder(w).Encode(map[string]any{"id": 1, "state": "pending"})
			}))
			defer srv.Close()
			app := NewApp()
			app.client = copr.New(srv.URL, nil)
			cmd := newBuildCmd(app)
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(io.Discard)
			args := append([]string{"submit", "owner/proj", "--output", "json"}, tt.args...)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute: %v", err)
			}
			var sent map[string]any
			if err := json.Unmarshal(gotBody, &sent); err != nil {
				t.Fatalf("decode submitted body: %v", err)
			}
			tt.check(t, sent)
		})
	}
}

func TestBuildSubmitBatchExclusive(t *testing.T) {
	app := NewApp()
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "scm", "--clone-url", "u",
		"--after-build-id", "1", "--with-build-id", "2", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a usage error for both batch options")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("exit code = %d, want usage (%d)", cerr.ExitCodeFor(err), cerr.ExitUsage)
	}
}

func TestSubmitInvocationFromSource(t *testing.T) {
	tests := []struct {
		name       string
		sourceType string
		dict       map[string]any
		want       string
	}{
		{
			name:       "scm stored shape",
			sourceType: "scm",
			dict: map[string]any{
				"type": "git", "clone_url": "https://github.com/abn/hello-rpm",
				"committish": "master", "spec": "hello.spec", "srpm_build_method": "rpkg",
			},
			want: "coprctl build submit REF --source scm --clone-url https://github.com/abn/hello-rpm --scm-type git --commit master --spec hello.spec --method rpkg",
		},
		{
			name:       "url stores the single url, not pkgs",
			sourceType: "link",
			dict:       map[string]any{"url": "https://example.com/hello.spec"},
			want:       "coprctl build submit REF --source url --url https://example.com/hello.spec",
		},
		{
			name:       "distgit custom clone url under clone_url",
			sourceType: "distgit",
			dict:       map[string]any{"package_name": "hello", "clone_url": "https://example.com/custom.git", "committish": "main"},
			want:       "coprctl build submit REF --source distgit --name hello --clone-url https://example.com/custom.git --commit main",
		},
		{
			name:       "pypi joins array python_versions",
			sourceType: "pypi",
			dict:       map[string]any{"pypi_package_name": "requests", "python_versions": []any{"3.9", "3.12"}},
			want:       "coprctl build submit REF --source pypi --pypi-name requests --python-versions 3.9,3.12",
		},
		{
			name:       "rubygems",
			sourceType: "rubygems",
			dict:       map[string]any{"gem_name": "rake"},
			want:       "coprctl build submit REF --source rubygems --gem rake",
		},
		{
			name:       "custom",
			sourceType: "custom",
			dict:       map[string]any{"script": "build.sh", "chroot": "fedora-42-x86_64"},
			want:       "coprctl build submit REF --source custom --script build.sh --script-chroot fedora-42-x86_64",
		},
		{
			name:       "unknown type emits the type only",
			sourceType: "upload",
			dict:       map[string]any{"url": "https://example.com/x.src.rpm"},
			want:       "coprctl build submit REF --source upload",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := submitInvocationFromSource(tt.sourceType, tt.dict)
			if got != tt.want {
				t.Errorf("submitInvocationFromSource = %q, want %q", got, tt.want)
			}
		})
	}
}

// reproduceServer serves the build-chroot list, a gzip log, and the source
// build config on one httptest server so result_url resolves locally.
// logStatus and configStatus override the log and config endpoint responses
// (200 serves the real bodies, anything else an error of that status).
func reproduceServer(t *testing.T, logStatus int, logContent string, configStatus int) *httptest.Server {
	t.Helper()
	var logBuf bytes.Buffer
	gz := gzip.NewWriter(&logBuf)
	_, _ = gz.Write([]byte(logContent))
	_ = gz.Close()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/build-chroot/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"name": "fedora-rawhide-x86_64", "state": "succeeded", "result_url": srv.URL + "/results/"},
				},
				"meta": map[string]any{},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/results/builder-live.log.gz":
			if logStatus != http.StatusOK {
				http.Error(w, `{"error": "not found"}`, logStatus)
				return
			}
			_, _ = w.Write(logBuf.Bytes())
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/build/source-build-config/42":
			if configStatus != http.StatusOK {
				http.Error(w, `{"error": "server error"}`, configStatus)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"source_type": "scm",
				"source_dict": map[string]any{
					"type": "git", "clone_url": "https://github.com/abn/hello-rpm",
					"committish": "master", "spec": "hello.spec", "srpm_build_method": "rpkg",
				},
				"memory_limit":  2048,
				"timeout":       18000,
				"is_background": false,
			})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

const recipeLine = "copr-rpmbuild --chroot fedora-rawhide-x86_64 --result /var/lib/mock/... --task-url https://example.com/task/42"

func TestBuildReproduceLogFirst(t *testing.T) {
	srv := reproduceServer(t, http.StatusOK, "info: "+recipeLine+"\n", http.StatusOK)
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"reproduce", "42/fedora-rawhide-x86_64", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	var res struct {
		Recipe     string         `json:"recipe"`
		TaskURL    string         `json:"task_url"`
		SourceType string         `json:"source_type"`
		SourceDict map[string]any `json:"source_dict"`
		Memory     *int           `json:"memory_limit"`
		Timeout    *int           `json:"timeout"`
		Background *bool          `json:"is_background"`
	}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if !strings.Contains(res.Recipe, "copr-rpmbuild") {
		t.Errorf("recipe = %q", res.Recipe)
	}
	if res.SourceType != "scm" {
		t.Errorf("source_type = %q", res.SourceType)
	}
	if res.SourceDict["srpm_build_method"] != "rpkg" {
		t.Errorf("source_dict = %v", res.SourceDict)
	}
	if res.Memory == nil || *res.Memory != 2048 {
		t.Errorf("memory_limit = %v", res.Memory)
	}
	if res.Background == nil || *res.Background {
		t.Errorf("is_background = %v", res.Background)
	}
}

func TestBuildReproduceFallbackToConfig(t *testing.T) {
	srv := reproduceServer(t, http.StatusOK, "some log without a recipe\n", http.StatusOK)
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"reproduce", "42/fedora-rawhide-x86_64", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reproduce should fall back to the source config, got: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# Source definition:") || !strings.Contains(out, "--source scm") {
		t.Errorf("fallback output = %q", out)
	}
}

func TestBuildReproduceFailsWithoutEither(t *testing.T) {
	srv := reproduceServer(t, http.StatusOK, "some log without a recipe\n", http.StatusNotFound)
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"reproduce", "42/fedora-rawhide-x86_64", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a failure with neither a log recipe nor a config")
	}
}

func TestBuildReproduceLogNotFoundFallsBackToConfig(t *testing.T) {
	srv := reproduceServer(t, http.StatusNotFound, "", http.StatusOK)
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"reproduce", "42/fedora-rawhide-x86_64", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reproduce should fall back to the source config, got: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "# Reproduce this build locally") {
		t.Errorf("log 404 must not print a recipe section, got %q", out)
	}
	if !strings.Contains(out, "# Source definition:") || !strings.Contains(out, "--source scm") {
		t.Errorf("log 404 fallback output = %q", out)
	}
}

func TestBuildReproduceSurfacesConfigFetchError(t *testing.T) {
	srv := reproduceServer(t, http.StatusOK, "info: "+recipeLine+"\n", http.StatusInternalServerError)
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"reproduce", "42/fedora-rawhide-x86_64", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a failure when the source config fetch errors")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitTransport {
		t.Errorf("exit code = %d, want transport (%d)", cerr.ExitCodeFor(err), cerr.ExitTransport)
	}
}

func TestBuildDeleteBatch(t *testing.T) {
	var calls int
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api_3/build/delete/list" {
			t.Errorf("got %s %s, want POST /api_3/build/delete/list", r.Method, r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{"builds": []int{1, 2}})
	}))
	defer srv.Close()
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"delete", "1", "2", "--yes", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want a single batch request", calls)
	}
	var sent map[string]any
	if err := json.Unmarshal(gotBody, &sent); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	ids, ok := sent["builds"].([]any)
	if !ok || len(ids) != 2 || ids[0] != float64(1) || ids[1] != float64(2) {
		t.Errorf("builds = %v", sent["builds"])
	}
}

func TestBuildDeleteRequiresYes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()
	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"delete", "1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected the --yes gate to block the delete")
	}
	if !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %q, want the --yes gate message", err)
	}
}

func TestBuildSubmitRpmUpload(t *testing.T) {
	var path, ct string
	var body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		ct = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": 77, "state": "pending"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	rpm := filepath.Join(dir, "hello-1.0-1.fc42.x86_64.rpm")
	if err := os.WriteFile(rpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "rpm-upload", "--rpm", rpm,
		"--chroot", "fedora-42-x86_64", "--sha256", "dae37be1717e714967b78e21ea9fdf00928a7652687d462f3ad631cde43d1a3d",
		"--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if path != "/api_3/build/create/rpm-upload" {
		t.Errorf("path = %q, want the rpm-upload endpoint", path)
	}
	if !strings.Contains(ct, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data", ct)
	}
	if !strings.Contains(body, "name=pkgs") || !strings.Contains(body, "application/x-rpm") ||
		!strings.Contains(body, "ownername") || !strings.Contains(body, "chroots") || !strings.Contains(body, "sha256") {
		t.Errorf("multipart body missing expected fields: %.300s", body)
	}
}

func TestBuildSubmitRpmUploadRequiresFlags(t *testing.T) {
	dir := t.TempDir()
	rpm := filepath.Join(dir, "x.rpm")
	if err := os.WriteFile(rpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing --rpm", args: []string{"submit", "owner/proj", "--source", "rpm-upload", "--chroot", "fedora-42-x86_64"}},
		{name: "missing --chroot", args: []string{"submit", "owner/proj", "--source", "rpm-upload", "--rpm", rpm}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := NewApp()
			cmd := newBuildCmd(app)
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(io.Discard)
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected a usage error")
			}
			if cerr.ExitCodeFor(err) != cerr.ExitUsage {
				t.Errorf("exit code = %d, want usage", cerr.ExitCodeFor(err))
			}
		})
	}
}

func TestBuildSubmitRpmUploadRejectsUnsupportedGenericFlags(t *testing.T) {
	dir := t.TempDir()
	rpm := filepath.Join(dir, "x.rpm")
	if err := os.WriteFile(rpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := NewApp()
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "rpm-upload", "--rpm", rpm,
		"--chroot", "fedora-42-x86_64", "--background"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a usage error for --background with rpm-upload")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("exit code = %d, want usage", cerr.ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "--background") {
		t.Errorf("error = %q, want it to name the unsupported flag", err.Error())
	}
}

func TestRpmUploadDisabledInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": "Direct RPM upload is not enabled on this Copr instance"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	rpm := filepath.Join(dir, "x.rpm")
	if err := os.WriteFile(rpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "rpm-upload", "--rpm", rpm,
		"--chroot", "fedora-42-x86_64", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a feature_disabled error")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitPrecondition {
		t.Errorf("exit code = %d, want precondition", cerr.ExitCodeFor(err))
	}
	var ce *cerr.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected cerr.Error, got %T", err)
	}
	if ce.Code != "feature_disabled" {
		t.Errorf("code = %q, want feature_disabled", ce.Code)
	}
	if !strings.Contains(ce.Hint, "DIRECT_RPM_UPLOAD") {
		t.Errorf("hint = %q, want a DIRECT_RPM_UPLOAD hint", ce.Hint)
	}
}

func TestRpmUploadOtherBadRequestPassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": "SHA256 mismatch"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	rpm := filepath.Join(dir, "x.rpm")
	if err := os.WriteFile(rpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newBuildCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"submit", "owner/proj", "--source", "rpm-upload", "--rpm", rpm,
		"--chroot", "fedora-42-x86_64", "--sha256", "00", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a bad_request error")
	}
	var ce *cerr.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected cerr.Error, got %T", err)
	}
	if ce.Code != "bad_request" {
		t.Errorf("code = %q, want bad_request", ce.Code)
	}
	if !strings.Contains(ce.Hint, "SHA256 mismatch") {
		t.Errorf("hint = %q, want the server SHA256 mismatch message", ce.Hint)
	}
}
