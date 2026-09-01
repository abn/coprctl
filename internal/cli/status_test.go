package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/copr"
)

func TestStatusSurfacesListBuildsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": 1, "name": "pkg-a", "source_type": "git"},
					{"id": 2, "name": "pkg-b", "source_type": "git"},
				},
				"meta": map[string]any{"limit": 100, "offset": 0, "order": "id", "order_type": "ASC"},
			})
		case "/api_3/build/list":
			if r.URL.Query().Get("packagename") == "pkg-b" {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{{"id": 42, "packagename": "pkg-a", "state": "succeeded"}},
				"meta":  map[string]any{"limit": 100, "offset": 0, "order": "id", "order_type": "ASC"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newStatusCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"owner/proj", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected an error exit for a package whose build list failed")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitBuildFailed {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitBuildFailed)
	}
	if !strings.Contains(buf.String(), `"state": "error"`) {
		t.Errorf("output missing error state:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"package": "pkg-b"`) {
		t.Errorf("output missing pkg-b entry:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), `"state": "succeeded"`) {
		t.Errorf("output missing pkg-a entry:\n%s", buf.String())
	}
}

func TestMonitorDefaultsBareRefOwner(t *testing.T) {
	var gotOwner, gotProject, gotDir string
	var gotFields []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/monitor" {
			http.NotFound(w, r)
			return
		}
		gotOwner = r.URL.Query().Get("ownername")
		gotProject = r.URL.Query().Get("projectname")
		gotDir = r.URL.Query().Get("project_dirname")
		gotFields = r.URL.Query()["additional_fields[]"]
		json.NewEncoder(w).Encode(map[string]any{"packages": []any{}})
	}))
	defer srv.Close()

	app := testAppWithProfile(t, "abn")
	app.client = copr.New(srv.URL, nil)
	cmd := newMonitorCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"hello-go", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if gotOwner != "abn" {
		t.Errorf("ownername = %q, want abn", gotOwner)
	}
	if gotProject != "hello-go" {
		t.Errorf("projectname = %q, want hello-go", gotProject)
	}
	if gotDir != "" {
		t.Errorf("project_dirname = %q, want empty for a bare ref", gotDir)
	}
	want := []string{"url_build_log", "url_backend_log"}
	if len(gotFields) != len(want) {
		t.Fatalf("additional_fields[] = %v, want %v", gotFields, want)
	}
	for i := range want {
		if gotFields[i] != want[i] {
			t.Errorf("additional_fields[%d] = %q, want %q (order matters)", i, gotFields[i], want[i])
		}
	}
}

func TestMonitorDirRefSendsProjectDirname(t *testing.T) {
	var gotDir string
	var gotFields []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/monitor" {
			http.NotFound(w, r)
			return
		}
		gotDir = r.URL.Query().Get("project_dirname")
		gotFields = r.URL.Query()["additional_fields[]"]
		json.NewEncoder(w).Encode(map[string]any{
			"packages": []any{
				map[string]any{
					"name": "hello",
					"chroots": map[string]any{
						"fedora-rawhide-x86_64": map[string]any{
							"state":           "succeeded",
							"status":          1,
							"build_id":        2926024,
							"url_build_log":   "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/builder-live.log.gz",
							"url_backend_log": "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/backend.log.gz",
							"pkg_version":     "2.10-1",
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newMonitorCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"devnullcake/hello-rpm:pr:123", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("monitor: %v", err)
	}
	if gotDir != "pr:123" {
		t.Errorf("project_dirname = %q, want pr:123", gotDir)
	}
	want := []string{"url_build_log", "url_backend_log"}
	if len(gotFields) != len(want) {
		t.Fatalf("additional_fields[] = %v, want %v", gotFields, want)
	}
	for i := range want {
		if gotFields[i] != want[i] {
			t.Errorf("additional_fields[%d] = %q, want %q (order matters)", i, gotFields[i], want[i])
		}
	}
	out := buf.String()
	for _, key := range []string{`"status": 1`, `"url_build_log": "https://`, `"url_backend_log": "https://`} {
		if !strings.Contains(out, key) {
			t.Errorf("json output missing %q:\n%s", key, out)
		}
	}
}

func TestMonitorHumanTable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/monitor" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"packages": []any{
				map[string]any{
					"name": "hello",
					"chroots": map[string]any{
						"fedora-rawhide-x86_64": map[string]any{
							"state":           "succeeded",
							"status":          1,
							"build_id":        2926024,
							"url_build_log":   "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/builder-live.log.gz",
							"url_backend_log": "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/backend.log.gz",
							"pkg_version":     "2.10-1",
						},
						"epel-9-x86_64": map[string]any{
							"state":       "waiting",
							"status":      9,
							"build_id":    0,
							"pkg_version": nil,
						},
					},
				},
			},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newMonitorCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"devnullcake/hello-rpm", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("monitor: %v", err)
	}
	out := buf.String()
	for _, h := range []string{"PACKAGE", "CHROOT", "STATE", "BUILD", "VERSION", "LOG"} {
		if !strings.Contains(out, h) {
			t.Errorf("table missing header %q:\n%s", h, out)
		}
	}
	// The raw int status stays out of the human table; STATE conveys it.
	if strings.Contains(out, "STATUS") {
		t.Errorf("table carries a raw STATUS column:\n%s", out)
	}
	if !strings.Contains(out, "2926024") {
		t.Errorf("table missing the succeeded build id:\n%s", out)
	}
	if !strings.Contains(out, "...") {
		t.Errorf("table missing the elided log url:\n%s", out)
	}
	// The waiting chroot has no build id or log; both render as "-".
	if !strings.Contains(out, "-\t") && !strings.Contains(out, "- ") {
		t.Errorf("table missing dash placeholders:\n%s", out)
	}
}

func TestElideLogURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "deep path keeps the build dir and filename",
			in:   "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/builder-live.log.gz",
			want: "https://download.copr-dev.fedorainfracloud.org/.../02926024-hello/builder-live.log.gz",
		},
		{
			name: "short url passes through",
			in:   "https://host.example/hello/builder-live.log.gz",
			want: "https://host.example/hello/builder-live.log.gz",
		},
		{
			name: "host only passes through",
			in:   "https://download.copr-dev.fedorainfracloud.org.long.example.invalid",
			want: "https://download.copr-dev.fedorainfracloud.org.long.example.invalid",
		},
		{
			name: "trailing slash is dropped",
			in:   "https://download.copr-dev.fedorainfracloud.org/results/devnullcake/hello-rpm/fedora-rawhide-x86_64/02926024-hello/builder-live.log.gz/",
			want: "https://download.copr-dev.fedorainfracloud.org/.../02926024-hello/builder-live.log.gz",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := elideLogURL(tc.in); got != tc.want {
				t.Errorf("elideLogURL() = %q, want %q", got, tc.want)
			}
		})
	}
}
