package copr

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
)

func testServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func TestListMockChroots(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/mock-chroots/list" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"fedora-rawhide-x86_64": "",
			"epel-9-x86_64":         "Builds against RHEL + EPEL",
		})
	})
	c := New(srv.URL, nil)
	ch, err := c.ListMockChroots(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ch) != 2 {
		t.Fatalf("got %d chroots, want 2", len(ch))
	}
	names := ch.ChrootNames()
	if names[0] != "epel-9-x86_64" || names[1] != "fedora-rawhide-x86_64" {
		t.Fatalf("sorted names = %v", names)
	}
}

func TestPaginationInternal(t *testing.T) {
	// Server returns pages of 2 until exhausted (5 items total). The client
	// must paginate internally until an empty page.
	items := []Project{
		{ID: 1, Name: "a", Ownername: "owner"},
		{ID: 2, Name: "b", Ownername: "owner"},
		{ID: 3, Name: "c", Ownername: "owner"},
		{ID: 4, Name: "d", Ownername: "owner"},
		{ID: 5, Name: "e", Ownername: "owner"},
	}
	pageSize := 2
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		off := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			off, _ = strconv.Atoi(v)
		}
		end := off + pageSize
		if end > len(items) {
			end = len(items)
		}
		page := items[off:end]
		if page == nil {
			page = []Project{}
		}
		json.NewEncoder(w).Encode(ProjectList{
			Items: page,
			Meta:  Meta{Limit: pageSize, Offset: off, Order: "id", OrderType: "ASC"},
		})
	})
	c := New(srv.URL, nil)
	got, err := c.ListProjects(context.Background(), "owner", pageSize)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d projects, want 5", len(got))
	}
}

func TestGetBuildAuthHeader(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "mylogin" || p != "mytoken" {
			t.Errorf("BasicAuth = (%q,%q,%v), want mylogin/mytoken", u, p, ok)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"id":           42,
			"state":        "succeeded",
			"chroots":      []string{"fedora-42-x86_64"},
			"submitted_on": 1700000000,
			"started_on":   1700000100,
			"ended_on":     1700000200,
			"ownername":    "owner",
			"projectname":  "proj",
		})
	})
	c := New(srv.URL, TokenAuth("mylogin", "mytoken"))
	b, err := c.GetBuild(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.ID != 42 || b.State != "succeeded" || len(b.Chroots) != 1 {
		t.Fatalf("build = %+v", b)
	}
	if !b.Started.IsSet || b.Started.Time().Unix() != 1700000100 {
		t.Fatalf("started timestamp not parsed: %+v", b.Started)
	}
}

func TestGetBuildAuthRequired(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	_, err := c.GetBuild(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected error for 401")
	}
	if cerr.ExitCodeFor(err) != 7 {
		t.Errorf("expected exit code 7 (auth), got %d", cerr.ExitCodeFor(err))
	}
}

func TestPermissionDeniedExitCode(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	_, err := c.GetBuild(context.Background(), 1)
	if err == nil {
		t.Fatalf("expected error for 403")
	}
	if cerr.ExitCodeFor(err) != 9 {
		t.Errorf("expected exit code 9 (permission), got %d", cerr.ExitCodeFor(err))
	}
}

func TestAuthCheck(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/auth-check" {
			t.Errorf("path = %s", r.URL.Path)
		}
		// Simulate a valid token.
		if u, p, ok := r.BasicAuth(); !ok || u != "l" || p != "t" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"name": "alice", "id": 42})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	id, err := c.AuthCheck(context.Background())
	if err != nil {
		t.Fatalf("auth-check: %v", err)
	}
	if id.Name != "alice" || id.ID != 42 {
		t.Errorf("identity = %+v", id)
	}
}

func TestAuthCheckInvalid(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{"error": "unauthorized"})
	})
	c := New(srv.URL, TokenAuth("bad", "bad"))
	if _, err := c.AuthCheck(context.Background()); err == nil {
		t.Fatal("expected auth error for bad token")
	}
}
