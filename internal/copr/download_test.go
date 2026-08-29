package copr

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestBuiltPackageFilename(t *testing.T) {
	tests := []struct {
		name string
		p    BuiltPackage
		want string
	}{
		{name: "plain", p: BuiltPackage{Name: "coprctl", Version: "0.4.7", Release: "1.fc42", Arch: "x86_64"}, want: "coprctl-0.4.7-1.fc42.x86_64.rpm"},
		{name: "noarch", p: BuiltPackage{Name: "coprctl", Version: "1.0", Release: "2", Arch: "noarch"}, want: "coprctl-1.0-2.noarch.rpm"},
		{name: "epoch ignored", p: BuiltPackage{Name: "pkg", Version: "2.0", Release: "3", Arch: "aarch64", Epoch: "1"}, want: "pkg-2.0-3.aarch64.rpm"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := BuiltPackageFilename(tt.p); got != tt.want {
				t.Errorf("BuiltPackageFilename = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetBuiltPackages(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api_3/build/built-packages/42" {
			t.Errorf("path = %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(BuiltPackages{
			"fedora-42-x86_64": {
				Packages: []BuiltPackage{{Name: "coprctl", Version: "0.4.7", Release: "1.fc42", Arch: "x86_64"}},
			},
			"epel-9-x86_64": {
				Packages: []BuiltPackage{{Name: "coprctl", Version: "0.4.7", Release: "1.el9", Arch: "aarch64"}},
			},
		})
	})
	c := New(srv.URL, nil)
	bp, err := c.GetBuiltPackages(context.Background(), 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(bp) != 2 {
		t.Fatalf("got %d chroots, want 2", len(bp))
	}
	if len(bp["fedora-42-x86_64"].Packages) != 1 {
		t.Fatalf("packages = %+v", bp["fedora-42-x86_64"].Packages)
	}
	got := BuiltPackageFilename(bp["fedora-42-x86_64"].Packages[0])
	if got != "coprctl-0.4.7-1.fc42.x86_64.rpm" {
		t.Errorf("filename = %q", got)
	}
}

func TestDownloadFileAuthAnd404(t *testing.T) {
	srv := testServer(t, func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != "l" || p != "t" {
			t.Errorf("BasicAuth = (%q,%q,%v), want l/t", u, p, ok)
		}
		switch r.URL.Path {
		case "/results/owner/proj/fedora-42-x86_64/pkg.rpm":
			w.Write([]byte("rpm-bytes"))
		default:
			http.NotFound(w, r)
		}
	})
	c := New(srv.URL, TokenAuth("l", "t"))
	dir := t.TempDir()
	if err := c.DownloadFile(context.Background(), srv.URL+"/results/owner/proj/fedora-42-x86_64/pkg.rpm", dir+"/pkg.rpm"); err != nil {
		t.Fatalf("download: %v", err)
	}
	err := c.DownloadFile(context.Background(), srv.URL+"/results/owner/proj/fedora-42-x86_64/missing.rpm", dir+"/missing.rpm")
	if err != ErrFileNotFound {
		t.Errorf("missing file error = %v, want ErrFileNotFound", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention not found: %v", err)
	}
}
