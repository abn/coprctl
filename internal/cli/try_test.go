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
