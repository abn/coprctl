package copr

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestGetProjectChrootBuildConfig(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/project-chroot/build-config" {
			t.Errorf("path = %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("ownername") != "owner" || q.Get("projectname") != "proj" || q.Get("chrootname") != "fedora-42-x86_64" {
			t.Errorf("query = %v", q)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"chroot": "fedora-42-x86_64",
			"repos":  []string{"copr"},
		})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	cfg, err := c.GetProjectChrootBuildConfig(context.Background(), "owner", "proj", "fedora-42-x86_64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg["chroot"] != "fedora-42-x86_64" {
		t.Errorf("chroot = %v", cfg["chroot"])
	}
}
