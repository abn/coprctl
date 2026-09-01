package forge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitLabHookLifecycle(t *testing.T) {
	hookID := int64(7)
	var created map[string]any
	var updated map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("PRIVATE-TOKEN") != "tok" {
			t.Errorf("missing auth: %q", r.Header.Get("PRIVATE-TOKEN"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks":
			json.NewEncoder(w).Encode([]map[string]any{{"id": hookID, "url": "https://copr/webhooks/gitlab/1/secret/"}})
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks":
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &created); err != nil {
				t.Errorf("create body: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": hookID, "url": "https://copr/webhooks/gitlab/1/secret/"})
		case r.Method == http.MethodPut && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks/7":
			data, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(data, &updated); err != nil {
				t.Errorf("update body: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks/7":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.EscapedPath() == "/projects/group%2Frepo/hooks/7/test/push_events":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"message": "201 Created"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	g := NewGitLab("tok", srv.URL)
	ctx := context.Background()

	hooks, err := g.ListHooks(ctx, "group", "repo")
	if err != nil || len(hooks) != 1 || hooks[0].ID != 7 {
		t.Fatalf("list hooks: %+v err=%v", hooks, err)
	}
	if got := hooks[0].DestinationURL(); got != "https://copr/webhooks/gitlab/1/secret/" {
		t.Errorf("DestinationURL = %q", got)
	}

	h, err := g.CreateHook(ctx, "group", "repo", "https://copr/webhooks/gitlab/1/secret/", HookOptions{TagOnly: true})
	if err != nil || h.ID != 7 {
		t.Fatalf("create hook: %+v err=%v", h, err)
	}
	// Tag-only maps to the boolean toggles, not an event list.
	if created["push_events"] != false {
		t.Errorf("create push_events = %v, want false for tag-only", created["push_events"])
	}
	if created["tag_push_events"] != true {
		t.Errorf("create tag_push_events = %v, want true", created["tag_push_events"])
	}

	if err := g.UpdateHook(ctx, "group", "repo", 7, "https://copr/webhooks/gitlab/1/secret/", HookOptions{TagOnly: false}); err != nil {
		t.Fatalf("update hook: %v", err)
	}
	if updated["push_events"] != true {
		t.Errorf("update push_events = %v, want true when branch pushes opt in", updated["push_events"])
	}

	status, err := g.TestHook(ctx, "group", "repo", 7, "push_events")
	if err != nil || status != http.StatusCreated {
		t.Fatalf("test hook: status=%d err=%v", status, err)
	}
	if err := g.DeleteHook(ctx, "group", "repo", 7); err != nil {
		t.Fatalf("delete hook: %v", err)
	}
}
