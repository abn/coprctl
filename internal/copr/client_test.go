package copr

import (
	"context"
	"encoding/json"
	"fmt"
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

// paginationServer serves `total` items in pages of `pageSize`, honoring the
// offset query param, regardless of the limit the client requests.
func paginationServer[T any](t *testing.T, total, pageSize int, mk func(int) T) *httptest.Server {
	t.Helper()
	return testServer(t, func(w http.ResponseWriter, r *http.Request) {
		off := 0
		if v := r.URL.Query().Get("offset"); v != "" {
			off, _ = strconv.Atoi(v)
		}
		end := off + pageSize
		if end > total {
			end = total
		}
		page := []T{}
		for i := off; i < end; i++ {
			page = append(page, mk(i+1))
		}
		json.NewEncoder(w).Encode(map[string]any{
			"items": page,
			"meta":  Meta{Limit: pageSize, Offset: off, Order: "id", OrderType: "ASC"},
		})
	})
}

func mkProject(i int) Project {
	return Project{ID: i, Name: fmt.Sprintf("p%d", i), Ownername: "owner"}
}

func mkBuild(i int) Build {
	return Build{ID: i, PackageName: fmt.Sprintf("p%d", i), State: "succeeded"}
}

func TestPaginationInternal(t *testing.T) {
	// The server has 10 projects in pages of 2, so it can always serve more
	// than the requested total. The client must stop at the total cap.
	srv := paginationServer(t, 10, 2, mkProject)
	c := New(srv.URL, nil)

	got, err := c.ListProjects(context.Background(), "owner", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("got %d projects, want 5 (total cap)", len(got))
	}
	if got[0].ID != 1 || got[4].ID != 5 {
		t.Fatalf("got ids %d..%d, want 1..5", got[0].ID, got[4].ID)
	}

	// limit <= 0 means "all".
	got, err = c.ListProjects(context.Background(), "owner", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("got %d projects, want 10 (all)", len(got))
	}
}

func TestListProjectsStopsOnEmptyPage(t *testing.T) {
	srv := paginationServer(t, 0, 2, mkProject)
	c := New(srv.URL, nil)
	got, err := c.ListProjects(context.Background(), "owner", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d projects, want 0", len(got))
	}
}

func TestListBuildsTotalCap(t *testing.T) {
	srv := paginationServer(t, 10, 2, mkBuild)
	c := New(srv.URL, nil)
	got, err := c.ListBuilds(context.Background(), "owner", "proj", "", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d builds, want 3 (total cap)", len(got))
	}
	if got[0].ID != 1 || got[2].ID != 3 {
		t.Fatalf("got ids %d..%d, want 1..3", got[0].ID, got[2].ID)
	}
}

func TestListBuildsAll(t *testing.T) {
	srv := paginationServer(t, 10, 2, mkBuild)
	c := New(srv.URL, nil)
	got, err := c.ListBuilds(context.Background(), "owner", "proj", "", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 10 {
		t.Fatalf("got %d builds, want 10 (all)", len(got))
	}
}

func TestListBuildsStopsOnEmptyPage(t *testing.T) {
	srv := paginationServer(t, 0, 2, mkBuild)
	c := New(srv.URL, nil)
	got, err := c.ListBuilds(context.Background(), "owner", "proj", "", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("got %d builds, want 0", len(got))
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
