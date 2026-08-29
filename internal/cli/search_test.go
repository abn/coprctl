package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestProjectSearchMapsToSearchProjects(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/project/search" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "acme" {
			t.Errorf("query = %q, want acme", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"name": "widgets", "ownername": "acme", "full_name": "acme/widgets", "description": "Widget builder"},
			},
		})
	}))
	defer srv.Close()

	app := NewApp()
	app.client = copr.New(srv.URL, nil)
	cmd := newProjectCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"search", "acme", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(buf.String(), "acme/widgets") {
		t.Errorf("output = %q, want matched project", buf.String())
	}
}
