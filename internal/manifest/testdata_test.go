package manifest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newManifestServer returns a server that serves a fixed project and package.
func newManifestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api_3/project/":
			json.NewEncoder(w).Encode(map[string]any{
				"name": "aetherpak", "ownername": "quadzero",
				"description": "live desc", "devel_mode": false,
				"chroot_repos": map[string]string{"fedora-42-x86_64": "u"},
			})
		case "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"name": "pkgo", "source_type": "scm", "auto_rebuild": true},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}
