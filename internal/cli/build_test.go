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
