package copr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/abn/coprctl/internal/cerr"
)

func testServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

// TestGetContextCancellation asserts that an in-flight read request is
// cancelled when its context is cancelled, rather than running out the client
// timeout. The server holds the request open until the client aborts it, so a
// request that is not context-aware would hang until the 60s timeout.
func TestGetContextCancellation(t *testing.T) {
	started := make(chan struct{})
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	})
	c := New(srv.URL, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := c.ListMockChroots(ctx); done <- err }()
	<-started
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error after cancel, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("request did not return promptly after context cancel")
	}
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
	return Build{ID: i, State: "succeeded"}
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
			"id":             42,
			"state":          "succeeded",
			"chroots":        []string{"fedora-42-x86_64"},
			"submitted_on":   1700000000,
			"started_on":     1700000100,
			"ended_on":       1700000200,
			"ownername":      "owner",
			"projectname":    "proj",
			"source_package": map[string]any{"name": "pkg", "version": "1.0", "url": "https://example.com/pkg"},
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
	if got := b.PackageName(); got != "pkg" {
		t.Errorf("PackageName() = %q, want pkg", got)
	}
	if !b.Started.IsSet || b.Started.Time().Unix() != 1700000100 {
		t.Fatalf("started timestamp not parsed: %+v", b.Started)
	}
}

func TestGetBuildDetail(t *testing.T) {
	buildRaw := readFixture(t, "testdata/build-2926020.json")
	listRaw := readFixture(t, "testdata/build-chroot-list.json")
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api_3/build/2926020":
			w.Header().Set("Content-Type", "application/json")
			w.Write(buildRaw)
		case r.URL.Path == "/api_3/build-chroot/list":
			if got := r.URL.Query().Get("build_id"); got != "2926020" {
				t.Errorf("build_id query = %q, want 2926020", got)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(listRaw)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	c := New(srv.URL, nil)
	b, err := c.GetBuildDetail(context.Background(), 2926020)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	states := b.ChrootStates()
	if states["epel-9-x86_64"] != "failed" {
		t.Errorf("ChrootStates()[epel-9-x86_64] = %q, want failed", states["epel-9-x86_64"])
	}
	if states["fedora-rawhide-x86_64"] != "succeeded" {
		t.Errorf("ChrootStates()[fedora-rawhide-x86_64] = %q, want succeeded", states["fedora-rawhide-x86_64"])
	}
}

func TestGetBuildDetailDegradesToBareBuild(t *testing.T) {
	buildRaw := readFixture(t, "testdata/build-source-package.json")
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api_3/build/2926022":
			w.Header().Set("Content-Type", "application/json")
			w.Write(buildRaw)
		case r.URL.Path == "/api_3/build-chroot/list":
			http.Error(w, "boom", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	c := New(srv.URL, nil)
	b, err := c.GetBuildDetail(context.Background(), 2926022)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := b.PackageName(); got != "hello" {
		t.Errorf("PackageName() = %q, want hello", got)
	}
	if b.Builds != nil {
		t.Errorf("builds = %v, want nil when build-chroot/list fails", b.Builds)
	}
}

func TestGetBuildDetailPermanentError(t *testing.T) {
	buildRaw := readFixture(t, "testdata/build-source-package.json")
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api_3/build/2926022":
			w.Header().Set("Content-Type", "application/json")
			w.Write(buildRaw)
		case r.URL.Path == "/api_3/build-chroot/list":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	})
	c := New(srv.URL, nil)
	if _, err := c.GetBuildDetail(context.Background(), 2926022); err == nil {
		t.Fatal("expected an error for a permanent build-chroot/list failure")
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

func TestNotFoundSurfacesAPIMessage(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "Group @foo does not exist"}`, http.StatusNotFound)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	_, err := c.GetProject(context.Background(), "@foo", "proj")
	if err == nil {
		t.Fatalf("expected 404 error")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitNotFound {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitNotFound)
	}
	var ce *cerr.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected cerr.Error, got %T", err)
	}
	if !strings.Contains(ce.Hint, "Group @foo does not exist") {
		t.Errorf("expected API message in hint, got %q", ce.Hint)
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
