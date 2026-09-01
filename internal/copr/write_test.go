package copr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func TestCreateProjectDeclaredSettingsPayload(t *testing.T) {
	var body map[string]any
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "proj"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	d := 7
	err := c.CreateProject(context.Background(), ProjectCreate{
		Owner: "owner", Name: "proj",
		Persistent:                 true,
		Storage:                    "pulp",
		AutoPrune:                  true,
		Bootstrap:                  "on",
		Isolation:                  "nspawn",
		ModuleHotfixes:             true,
		Appstream:                  true,
		PackitForgeProjectsAllowed: []string{"github.com/quadzero/aetherpak"},
		FollowFedoraBranching:      true,
		RepoPriority:               50,
		Multilib:                   true,
		FedoraReview:               true,
		RuntimeDependencies:        []string{"https://repo.example.com/fedora/"},
		DeleteAfterDays:            &d,
	}, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{
		"persistent":                    true,
		"storage":                       "pulp",
		"auto_prune":                    true,
		"bootstrap":                     "on",
		"isolation":                     "nspawn",
		"module_hotfixes":               true,
		"appstream":                     true,
		"packit_forge_projects_allowed": []any{"github.com/quadzero/aetherpak"},
		"follow_fedora_branching":       true,
		"repo_priority":                 float64(50),
		"multilib":                      true,
		"fedora_review":                 true,
		"runtime_dependencies":          []any{"https://repo.example.com/fedora/"},
		"delete_after_days":             float64(7),
	}
	for k, wantV := range want {
		if gotV, ok := body[k]; !ok || fmt.Sprint(gotV) != fmt.Sprint(wantV) {
			t.Errorf("create body[%q] = %v (%v), want %v", k, gotV, ok, wantV)
		}
	}
}

func TestCreateProjectOmitsUndeclaredSettings(t *testing.T) {
	// A minimal create must not send implicit zero defaults: auto_prune:false
	// and persistent:false trip the upstream admin-only exceptions at create.
	var body map[string]any
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "proj"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	if err := c.CreateProject(context.Background(), ProjectCreate{Owner: "owner", Name: "proj"}, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"persistent", "storage", "auto_prune", "bootstrap",
		"isolation", "module_hotfixes", "appstream", "packit_forge_projects_allowed",
		"follow_fedora_branching", "repo_priority", "multilib", "fedora_review",
		"runtime_dependencies", "delete_after_days"} {
		if _, ok := body[k]; ok {
			t.Errorf("undeclared field %q sent on create: %v", k, body[k])
		}
	}
}

func TestEditProjectDeclaredSettingsPayload(t *testing.T) {
	var body map[string]any
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s", r.Method)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "proj"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	true_, false_ := true, false
	d := 30
	err := c.EditProject(context.Background(), ProjectEdit{
		Owner: "owner", Project: "proj",
		AutoPrune:                  &true_,
		Bootstrap:                  "on",
		Isolation:                  "nspawn",
		ModuleHotfixes:             &true_,
		Appstream:                  &false_,
		PackitForgeProjectsAllowed: []string{"github.com/quadzero/aetherpak"},
		FollowFedoraBranching:      &false_,
		RepoPriority:               42,
		UnlistedOnHomepage:         &true_,
		Multilib:                   &true_,
		FedoraReview:               &false_,
		RuntimeDependencies:        []string{"https://repo.example.com/fedora/"},
		DeleteAfterDays:            &d,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]any{
		"auto_prune":                    true,
		"bootstrap":                     "on",
		"isolation":                     "nspawn",
		"module_hotfixes":               true,
		"appstream":                     false,
		"packit_forge_projects_allowed": []any{"github.com/quadzero/aetherpak"},
		"follow_fedora_branching":       false,
		"repo_priority":                 float64(42),
		"unlisted_on_hp":                true,
		"multilib":                      true,
		"fedora_review":                 false,
		"runtime_dependencies":          []any{"https://repo.example.com/fedora/"},
		"delete_after_days":             float64(30),
	}
	for k, wantV := range want {
		if gotV, ok := body[k]; !ok || fmt.Sprint(gotV) != fmt.Sprint(wantV) {
			t.Errorf("edit body[%q] = %v (%v), want %v", k, gotV, ok, wantV)
		}
	}
	for _, k := range []string{"persistent", "storage"} {
		if _, ok := body[k]; ok {
			t.Errorf("create-only field %q sent on edit: %v", k, body[k])
		}
	}
}

func TestPackageCreateSettingsPayload(t *testing.T) {
	var body map[string]any
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/package/add/owner/proj/pkgo/scm" {
			t.Errorf("path = %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "pkgo"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	maxB, timeout := 5, 3600
	err := c.CreatePackage(context.Background(), PackageCreate{
		Owner: "owner", Project: "proj", Name: "pkgo",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
		MaxBuilds: &maxB, Timeout: &timeout,
		ChrootDenylist: []string{"fedora-rawhide-*", "epel-9-*"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["max_builds"] != float64(5) {
		t.Errorf("max_builds = %v", body["max_builds"])
	}
	if body["timeout"] != float64(3600) {
		t.Errorf("timeout = %v", body["timeout"])
	}
	if body["chroot_denylist"] != "fedora-rawhide-*,epel-9-*" {
		t.Errorf("chroot_denylist = %v, want comma-joined", body["chroot_denylist"])
	}
}

func TestPackageCreateOmitsNilSettings(t *testing.T) {
	var body map[string]any
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "pkgo"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	if err := c.CreatePackage(context.Background(), PackageCreate{
		Owner: "owner", Project: "proj", Name: "pkgo",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "u"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, k := range []string{"max_builds", "timeout", "chroot_denylist"} {
		if _, ok := body[k]; ok {
			t.Errorf("nil setting %q sent: %v", k, body[k])
		}
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

func TestCreatePackageAutoRebuildPayload(t *testing.T) {
	var seenWebhookRebuild *bool
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if v, ok := body["webhook_rebuild"]; ok {
			b := v.(bool)
			seenWebhookRebuild = &b
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "pkgo"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))

	// SetAutoRebuild true must emit webhook_rebuild with the declared value.
	if err := c.CreatePackage(context.Background(), PackageCreate{
		Owner: "owner", Project: "proj", Name: "pkgo",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
		AutoRebuild: true, SetAutoRebuild: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenWebhookRebuild == nil || !*seenWebhookRebuild {
		t.Errorf("webhook_rebuild not sent when SetAutoRebuild is true")
	}

	// SetAutoRebuild false (an edit not touching the flag) must not clobber it.
	seenWebhookRebuild = nil
	if err := c.CreatePackage(context.Background(), PackageCreate{
		Owner: "owner", Project: "proj", Name: "pkgo",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
		AutoRebuild: true,
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seenWebhookRebuild != nil {
		t.Errorf("webhook_rebuild sent when SetAutoRebuild is false: %v", *seenWebhookRebuild)
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
	builds, err := c.SubmitBuild(context.Background(), BuildSubmit{
		Owner: "owner", Project: "proj",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
		Chroots: []string{"fedora-42-x86_64"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(builds) != 1 || builds[0].ID != 123 || builds[0].State != "pending" {
		t.Fatalf("builds = %+v", builds)
	}
}

func TestSubmitBuildURLDecodesEnvelope(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/build/create/url" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if pkgs, ok := body["pkgs"].([]any); !ok || len(pkgs) != 1 {
			t.Errorf("pkgs = %v", body["pkgs"])
		}
		// The url endpoint wraps every build (even a single one) in an items
		// envelope; decoding it as one flat build would yield an all-zero id.
		json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"id": 1, "state": "pending"},
				{"id": 2, "state": "pending"},
			},
			"meta": map[string]any{},
		})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	builds, err := c.SubmitBuild(context.Background(), BuildSubmit{
		Owner: "owner", Project: "proj",
		SourceType: SourceURL, Source: map[string]any{"pkgs": []string{"https://example.com/a.spec"}},
		Chroots: []string{"fedora-42-x86_64"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(builds) != 2 || builds[0].ID != 1 || builds[1].ID != 2 {
		t.Fatalf("builds = %+v, want both envelope items", builds)
	}
}

func TestSubmitBuildURLEmptyEnvelopeErrors(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"items": []any{}, "meta": map[string]any{}})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	_, err := c.SubmitBuild(context.Background(), BuildSubmit{
		Owner: "owner", Project: "proj",
		SourceType: SourceURL, Source: map[string]any{"pkgs": []string{"https://example.com/a.spec"}},
	})
	if err == nil {
		t.Fatal("expected an error for an empty items envelope")
	}
}

func TestSubmitBuildFlatEmptyBuildErrors(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	_, err := c.SubmitBuild(context.Background(), BuildSubmit{
		Owner: "owner", Project: "proj",
		SourceType: SourceSCM, Source: map[string]any{"clone_url": "https://example.com/r.git"},
	})
	if err == nil {
		t.Fatal("expected an error for an empty flat build")
	}
}

func TestRebuildPackage(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api_3/package/list":
			json.NewEncoder(w).Encode(map[string]any{
				"items": []map[string]any{
					{"id": 1, "name": "pkgo", "source_type": "scm", "source_dict": map[string]any{"clone_url": "https://example.com/r.git"}},
				},
				"meta": map[string]any{"limit": 100, "offset": 0},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api_3/build/create/scm":
			json.NewEncoder(w).Encode(map[string]any{"id": 456, "state": "pending"})
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	builds, err := c.RebuildPackage(context.Background(), "owner", "proj", "pkgo", []string{"fedora-42-x86_64"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(builds) != 1 || builds[0].ID != 456 {
		t.Fatalf("builds = %+v, want the single rebuilt build", builds)
	}
}

func TestDeletePackageRouteAndBody(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/api_3/package/delete" {
			t.Errorf("path = %s, want the fixed /package/delete route", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["ownername"] != "owner" || body["projectname"] != "proj" || body["package_name"] != "pkgo" {
			t.Errorf("body = %v, want ownername/projectname/package_name", body)
		}
		w.WriteHeader(http.StatusOK)
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	if err := c.DeletePackage(context.Background(), "owner", "proj", "pkgo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
		if calls == 1 {
			// The add conflicts; upsert must fall back to an edit so package
			// settings reach the existing package.
			http.Error(w, `{"error": "Package already exists in this project."}`, http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/api_3/package/edit/o/p/x/scm" {
			t.Errorf("fallback path = %s, want package edit", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["max_builds"] != float64(5) {
			t.Errorf("edit max_builds = %v", body["max_builds"])
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{"name": "x"})
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	m := 5
	if err := c.UpsertPackage(context.Background(), PackageCreate{
		Owner: "o", Project: "p", Name: "x", SourceType: SourceSCM,
		Source: map[string]any{"clone_url": "u"}, MaxBuilds: &m,
	}); err != nil {
		t.Fatalf("upsert should fall back to edit on already-exists: %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls (add then edit), got %d", calls)
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
	b, err := c.UploadBuild(context.Background(), "owner", "proj", srpm, "")
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if b.ID != 99 {
		t.Errorf("build id = %d", b.ID)
	}
}

func TestDirnameFor(t *testing.T) {
	if got := dirnameFor("proj", "testing"); got != "proj:testing" {
		t.Errorf("dirnameFor = %q", got)
	}
	if got := dirnameFor("proj", "pr:123"); got != "proj:pr:123" {
		t.Errorf("dirnameFor pr = %q", got)
	}
}
