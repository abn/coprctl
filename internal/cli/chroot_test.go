package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/chroot"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/state"
)

func TestWarnIfInactive(t *testing.T) {
	var buf bytes.Buffer
	cmd := newChrootCmd(NewApp())
	cmd.SetErr(&buf)

	// EOL chroot in catalog: warns.
	catalog := copr.MockChroots{
		"fedora-42-x86_64": "Fedora 42",
		"fedora-43-x86_64": "Fedora 43",
	}
	cmd.SetArgs(nil)
	warnIfInactive(cmd, "fedora-42-x86_64", catalog)
	if !strings.Contains(buf.String(), "preserved") {
		t.Errorf("expected preserved warning, got %q", buf.String())
	}

	// Active chroot: no warning.
	buf.Reset()
	warnIfInactive(cmd, "fedora-43-x86_64", catalog)
	if buf.Len() != 0 {
		t.Errorf("expected no warning for active chroot, got %q", buf.String())
	}
}

func TestFilterByState(t *testing.T) {
	states := []chroot.Info{
		{Name: "a", State: chroot.Active},
		{Name: "b", State: chroot.Preserved},
	}
	got := filterByState(states, "preserved")
	if len(got) != 1 || got[0].Name != "b" {
		t.Errorf("filterByState = %+v", got)
	}
}

func TestLoadChrootCatalogLiveSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{
			"fedora-rawhide-x86_64": "",
			"epel-9-x86_64":         "Builds against RHEL + EPEL",
		})
	}))
	defer srv.Close()
	c := copr.New(srv.URL, nil)
	cacheDir := filepath.Join(t.TempDir(), "cache")

	got, warn, err := loadChrootCatalog(context.Background(), c, cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warn != "" {
		t.Errorf("unexpected warning %q", warn)
	}
	if len(got) != 2 {
		t.Fatalf("got %d chroots, want 2", len(got))
	}
	// The success path must store the cache for later offline use.
	cached, ok := state.NewChrootCache(cacheDir).Load()
	if !ok {
		t.Fatal("expected catalog cached after live success")
	}
	if len(*cached) != 2 {
		t.Fatalf("cached %d chroots, want 2", len(*cached))
	}
}

func TestLoadChrootCatalogFallsBackToCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := copr.New(srv.URL, nil)

	cacheDir := filepath.Join(t.TempDir(), "cache")
	want := copr.MockChroots{"fedora-44-x86_64": "Fedora 44 x86_64"}
	if err := state.NewChrootCache(cacheDir).Store(want); err != nil {
		t.Fatal(err)
	}

	got, warn, err := loadChrootCatalog(context.Background(), c, cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(warn, "could not reach the instance") {
		t.Errorf("warning = %q, want offline notice", warn)
	}
	if !strings.Contains(warn, cacheDir) {
		t.Errorf("warning = %q, want cache path", warn)
	}
	if len(got) != 1 || got["fedora-44-x86_64"] != "Fedora 44 x86_64" {
		t.Errorf("got %v, want cached catalog %v", got, want)
	}
}

func TestLoadChrootCatalogNoCacheReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()
	c := copr.New(srv.URL, nil)

	got, warn, err := loadChrootCatalog(context.Background(), c, filepath.Join(t.TempDir(), "cache"))
	if err == nil {
		t.Fatal("expected the original fetch error, got nil")
	}
	if got != nil {
		t.Errorf("expected no chroots, got %v", got)
	}
	if warn != "" {
		t.Errorf("unexpected warning %q", warn)
	}
}
