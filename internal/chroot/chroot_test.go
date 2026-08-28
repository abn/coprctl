package chroot

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		name        string
		distro, ver string
		arch        string
		wantErr     bool
	}{
		{"fedora-rawhide-x86_64", "fedora", "rawhide", "x86_64", false},
		{"fedora-42-x86_64", "fedora", "42", "x86_64", false},
		{"epel-9-x86_64", "epel", "9", "x86_64", false},
		{"centos-stream-10-x86_64", "centos-stream", "10", "x86_64", false},
		{"opensuse-leap-15.6-x86_64", "opensuse-leap", "15.6", "x86_64", false},
		{"malformed", "", "", "", true},
		{"noarch", "", "", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.name)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) expected error, got %+v", tc.name, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.name, err)
			}
			if got.Distro != tc.distro || got.Version != tc.ver || got.Arch != tc.arch {
				t.Errorf("Parse(%q) = %+v, want %s/%s/%s", tc.name, got, tc.distro, tc.ver, tc.arch)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	// Pin "today" so EOL classification is deterministic.
	now = func() time.Time { return time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC) }
	defer func() { now = time.Now }()

	cases := []struct {
		name      string
		inCatalog bool
		want      State
	}{
		// 42 went EOL 2026-06-16.
		{"fedora-42-x86_64", true, Preserved},
		// 43 is EOL 2026-12-01, still active today.
		{"fedora-43-x86_64", true, Active},
		// rawhide never EOL.
		{"fedora-rawhide-x86_64", true, Active},
		// EPEL 8 EOL 2029.
		{"epel-8-x86_64", true, Active},
		// Not in the catalog is Deleted regardless of name.
		{"fedora-40-x86_64", false, Deleted},
		// Unknown release stays Active rather than being guessed EOL.
		{"fedora-45-x86_64", true, Active},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.name, tc.inCatalog); got != tc.want {
				t.Errorf("Classify(%q, %v) = %q, want %q", tc.name, tc.inCatalog, got, tc.want)
			}
		})
	}
}
