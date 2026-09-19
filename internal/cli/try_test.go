package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveChrootImage(t *testing.T) {
	tests := []struct {
		chroot     string
		match      string
		image      string
		confidence string
	}{
		{"fedora-44-x86_64", "exact", "quay.io/abn/rpmbuilder:fedora-44", "medium"},
		{"fedora-rawhide-x86_64", "exact", "quay.io/abn/rpmbuilder:fedora-rawhide", "medium"},
		{"epel-9-x86_64", "substitute", "quay.io/abn/rpmbuilder:rockylinux-9", "low"},
		{"centos-stream-10-x86_64", "substitute", "quay.io/abn/rpmbuilder:rockylinux-10", "low"},
		{"opensuse-leap-15.6-x86_64", "none", "", ""},
		{"malformed", "none", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.chroot, func(t *testing.T) {
			got := resolveChrootImage(tt.chroot)
			if got.Match != tt.match {
				t.Errorf("Match = %q, want %q", got.Match, tt.match)
			}
			if got.Image != tt.image {
				t.Errorf("Image = %q, want %q", got.Image, tt.image)
			}
			if got.Confidence != tt.confidence {
				t.Errorf("Confidence = %q, want %q", got.Confidence, tt.confidence)
			}
		})
	}
}

func TestFindSRPM(t *testing.T) {
	dir := t.TempDir()
	if _, err := findSRPM(dir); err == nil {
		t.Fatal("expected error with no SRPMs")
	}
	a := filepath.Join(dir, "old-0.1-1.src.rpm")
	b := filepath.Join(dir, "new-0.2-1.src.rpm")
	if err := os.WriteFile(a, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// b has a newer mtime (created after a), so findSRPM should pick b.
	got, err := findSRPM(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != b {
		t.Errorf("findSRPM = %q, want %q", got, b)
	}
}

func TestPreflightStatusIncludesReason(t *testing.T) {
	status, reason := preflightStatus(nil)
	if status != "passed" || reason != "" {
		t.Errorf("preflightStatus(nil) = (%q,%q), want (passed,\"\")", status, reason)
	}
	status, reason = preflightStatus(fmt.Errorf("mock failure: buildroot exploded"))
	if status != "failed" || reason != "mock failure: buildroot exploded" {
		t.Errorf("preflightStatus(err) = (%q,%q), want (failed, error text)", status, reason)
	}
}

func TestPreflightStatusTruncatesReason(t *testing.T) {
	long := strings.Repeat("x", 500)
	status, reason := preflightStatus(errors.New(long))
	if status != "failed" {
		t.Errorf("status = %q, want failed", status)
	}
	if len(reason) != maxReasonLen+len("...") {
		t.Errorf("reason length = %d, want %d", len(reason), maxReasonLen+len("..."))
	}
	if !strings.HasPrefix(reason, long[:maxReasonLen]) {
		t.Error("reason does not start with the truncated error text")
	}
	if !strings.HasSuffix(reason, "...") {
		t.Error("reason missing truncation suffix")
	}
}

func TestFindSpecTemplateGuidesToMakeSrpm(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pkg.spec.in"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findSpec(dir); err == nil {
		t.Fatal("expected error for template-only dir")
	} else if !strings.Contains(err.Error(), "make_srpm") {
		t.Errorf("error = %q, want a pointer at make_srpm", err)
	} else if !strings.Contains(err.Error(), "render it first") {
		t.Errorf("error = %q, want the local render step first", err)
	}
}

func TestFindSpecPointsAtSubdir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "packaging", "rpm")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "pkg.spec"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findSpec(dir); err == nil {
		t.Fatal("expected error for empty top dir")
	} else if !strings.Contains(err.Error(), sub) {
		t.Errorf("error = %q, want it to name %q", err, sub)
	}
}

func TestFindSpecSubdirTemplate(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "packaging", "rpm")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "pkg.spec.in"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findSpec(dir); err == nil {
		t.Fatal("expected error for template-only tree")
	} else if !strings.Contains(err.Error(), "render it first") {
		t.Errorf("error = %q, want the local render step", err)
	}
}

func TestFindSpecPlainBeatsDistantTemplate(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"rpm", filepath.Join("packaging", "rpm")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "packaging", "rpm", "pkg.spec.in"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rpm", "pkg.spec"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findSpec(dir); err == nil {
		t.Fatal("expected error for empty top dir")
	} else if !strings.Contains(err.Error(), filepath.Join(dir, "rpm")) {
		t.Errorf("error = %q, want the usable plain spec named", err)
	}
}

func TestFindSpecTopTemplateYieldsToSubdirPlain(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "packaging", "rpm")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pkg.spec.in"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "pkg.spec"), []byte("Name: pkg\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := findSpec(dir); err == nil {
		t.Fatal("expected error for empty top dir")
	} else if !strings.Contains(err.Error(), sub) {
		t.Errorf("error = %q, want the subdir plain spec named, not the top template", err)
	}
}
