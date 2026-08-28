package copr

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
)

func TestCreateProjectJSONBody(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/project/add/owner" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q", ct)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "proj" {
			t.Errorf("name = %v", body["name"])
		}
		if ch, ok := body["chroots"].([]any); !ok || len(ch) != 1 || ch[0] != "fedora-42-x86_64" {
			t.Errorf("chroots = %v", body["chroots"])
		}
		if body["devel_mode"] != true {
			t.Errorf("devel_mode = %v", body["devel_mode"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "proj"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	err := c.CreateProject(context.Background(), ProjectCreate{
		Owner: "owner", Name: "proj", Chroots: []string{"fedora-42-x86_64"}, DevelMode: true,
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreatePackageFieldName(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/package/add/owner/proj/pkgo/scm" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["package_name"] != "pkgo" {
			t.Errorf("package_name = %v (must use underscore form)", body["package_name"])
		}
		if body["clone_url"] != "https://example.com/r.git" {
			t.Errorf("clone_url = %v", body["clone_url"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "pkgo"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	err := c.CreatePackage(context.Background(), PackageCreate{
		Owner: "owner", Project: "proj", Name: "pkgo",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitBuildPayload(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/build/create/scm" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["ownername"] != "owner" || body["projectname"] != "proj" {
			t.Errorf("owner/project = %v/%v", body["ownername"], body["projectname"])
		}
		if cn, ok := body["chroot_names"].([]any); !ok || len(cn) != 1 {
			t.Errorf("chroot_names = %v", body["chroot_names"])
		}
		if body["clone_url"] != "https://example.com/r.git" {
			t.Errorf("clone_url = %v", body["clone_url"])
		}
		json.NewEncoder(w).Encode(map[string]any{"id": 123, "state": "pending"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	b, err := c.SubmitBuild(context.Background(), BuildSubmit{
		Owner: "owner", Project: "proj",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
		Chroots: []string{"fedora-42-x86_64"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.ID != 123 || b.State != "pending" {
		t.Fatalf("build = %+v", b)
	}
}

func TestDeleteProjectVerifyPayload(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/api_3/project/delete/") {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["verify"] != true {
			t.Errorf("verify = %v", body["verify"])
		}
		w.WriteHeader(http.StatusOK)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	if err := c.DeleteProject(context.Background(), "owner", "proj"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateProjectIfNotExists(t *testing.T) {
	// First request returns 409 conflict; with existOK the client must treat
	// it as success. Without existOK it must surface the error.
	calls := 0
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, `{"error": "Project with this name already exists."}`, http.StatusConflict)
	})
	c := New(srv.URL, TokenAuth("l", "t"))

	// existOK=false must return an error with code "conflict".
	err := c.CreateProject(context.Background(), ProjectCreate{Owner: "o", Name: "p"}, false)
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if cerr.ExitCodeFor(err) != 10 {
		t.Errorf("expected exit code 10 (conflict), got %d", cerr.ExitCodeFor(err))
	}

	// existOK=true must succeed.
	if err := c.CreateProject(context.Background(), ProjectCreate{Owner: "o", Name: "p"}, true); err != nil {
		t.Fatalf("existOK should suppress conflict, got: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestUpsertPackageToleratesExisting(t *testing.T) {
	calls := 0
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, `{"error": "Package already exists in this project."}`, http.StatusBadRequest)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	if err := c.UpsertPackage(context.Background(), PackageCreate{
		Owner: "o", Project: "p", Name: "x", SourceType: SourceSCM,
		Source: map[string]any{"clone_url": "u"},
	}); err != nil {
		t.Fatalf("upsert should tolerate already-exists: %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestUpsertPackageSurfacesOtherErrors(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "some other problem"}`, http.StatusBadRequest)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	if err := c.UpsertPackage(context.Background(), PackageCreate{
		Owner: "o", Project: "p", Name: "x", SourceType: SourceSCM,
		Source: map[string]any{"clone_url": "u"},
	}); err == nil {
		t.Fatal("expected error for non-already-exists 400")
	}
}

func TestBadRequestSurfacesAPIMessage(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error": "name: This field is required."}`, http.StatusBadRequest)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	err := c.CreateProject(context.Background(), ProjectCreate{Owner: "o"}, false)
	if err == nil {
		t.Fatalf("expected bad request error")
	}
	var ce *cerr.Error
	if !errors.As(err, &ce) {
		t.Fatalf("expected cerr.Error, got %T", err)
	}
	if ce.Hint == "" || !strings.Contains(ce.Hint, "name: This field is required") {
		t.Errorf("expected API message in hint, got %q", ce.Hint)
	}
}

func TestRotateAPIToken(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api_3/api-token" {
			t.Errorf("got %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{
			"api_login": "newlogin", "api_token": "newtoken", "expiration": "2027-03-01",
		})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	nt, err := c.RotateAPIToken(context.Background())
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if nt.APILogin != "newlogin" || nt.APIToken != "newtoken" || nt.Expiration != "2027-03-01" {
		t.Errorf("token = %+v", nt)
	}
}

func TestEditProjectChroots(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/project/edit/owner/proj" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		cn, ok := body["chroots"].([]any)
		if !ok || len(cn) != 2 {
			t.Errorf("chroots = %v", body["chroots"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "proj"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	chroots := []string{"fedora-42-x86_64", "epel-9-x86_64"}
	if err := c.EditProject(context.Background(), ProjectEdit{
		Owner: "owner", Project: "proj", Chroots: &chroots,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUploadBuildPayload(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/build/create/upload" {
			t.Errorf("path = %s", r.URL.Path)
		}
		ct := r.Header.Get("Content-Type")
		if !strings.Contains(ct, "multipart/form-data") {
			t.Errorf("Content-Type = %q, want multipart", ct)
		}
		// Verify the multipart body has both a json part and a pkgs part.
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		s := string(body)
		if !strings.Contains(s, `name="json"`) || !strings.Contains(s, `name="pkgs"`) {
			t.Errorf("multipart body missing json/pkgs parts: %.300s", s)
		}
		if !strings.Contains(s, "ownername") {
			t.Errorf("json part missing ownername: %.300s", s)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"id": 99, "state": "pending"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	// Write a temp srpm file.
	dir := t.TempDir()
	srpm := filepath.Join(dir, "x.src.rpm")
	if err := os.WriteFile(srpm, []byte("fake rpm"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := c.UploadBuild(context.Background(), "owner", "proj", srpm)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if b.ID != 99 {
		t.Errorf("build id = %d", b.ID)
	}
}
