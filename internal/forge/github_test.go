package forge

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubHookLifecycle(t *testing.T) {
	hookID := int64(42)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("missing auth: %q", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/hooks":
			json.NewEncoder(w).Encode([]map[string]any{{"id": hookID, "url": "u", "active": true, "events": []string{"push"}}})
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/hooks":
			json.NewEncoder(w).Encode(map[string]any{"id": hookID, "url": "u", "active": true, "events": []string{"push"}})
		case r.Method == http.MethodPatch && r.URL.Path == "/repos/o/r/hooks/42":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/repos/o/r/hooks/42":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/repos/o/r/hooks/42/pings":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/repos/o/r/hooks/42/deliveries":
			json.NewEncoder(w).Encode([]map[string]any{{"status": "OK", "status_code": 200}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := NewGitHub("tok")
	g.HTTP = srv.Client()
	// Override base URL for the test.
	g.baseURL = srv.URL

	ctx := context.Background()
	hooks, err := g.ListHooks(ctx, "o", "r")
	if err != nil || len(hooks) != 1 || hooks[0].ID != 42 {
		t.Fatalf("list hooks: %+v err=%v", hooks, err)
	}
	h, err := g.CreateHook(ctx, "o", "r", "https://copr/webhooks/github/1/secret/", []string{"push"})
	if err != nil || h.ID != 42 {
		t.Fatalf("create hook: %+v err=%v", h, err)
	}
	if err := g.UpdateHook(ctx, "o", "r", 42, "https://copr/webhooks/github/1/secret/", []string{"push"}); err != nil {
		t.Fatalf("update hook: %v", err)
	}
	code, err := g.PingHook(ctx, "o", "r", 42)
	if err != nil || code != 200 {
		t.Fatalf("ping hook: code=%d err=%v", code, err)
	}
	if err := g.DeleteHook(ctx, "o", "r", 42); err != nil {
		t.Fatalf("delete hook: %v", err)
	}
}
