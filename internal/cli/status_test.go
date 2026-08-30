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
	var gotOwner, gotProject string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/monitor" {
			http.NotFound(w, r)
			return
		}
		gotOwner = r.URL.Query().Get("ownername")
		gotProject = r.URL.Query().Get("projectname")
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
}
