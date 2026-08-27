package cli

import "testing"

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
