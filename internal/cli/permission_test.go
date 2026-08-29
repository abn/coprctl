package cli

import (
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestParsePermissionState(t *testing.T) {
	cases := []struct {
		role, value string
		want        copr.PermissionState
		wantErr     bool
	}{
		{"admin", "", "", false},
		{"admin", "nothing", copr.PermissionNothing, false},
		{"admin", "request", copr.PermissionRequest, false},
		{"builder", "approved", copr.PermissionApproved, false},
		{"builder", "granted", "", true},
		{"admin", "approve", "", true},
	}
	for _, tc := range cases {
		got, err := parsePermissionState(tc.role, tc.value)
		if tc.wantErr {
			if err == nil {
				t.Errorf("parsePermissionState(%q, %q): expected error", tc.role, tc.value)
			}
			continue
		}
		if err != nil {
			t.Errorf("parsePermissionState(%q, %q): unexpected error: %v", tc.role, tc.value, err)
		}
		if got != tc.want {
			t.Errorf("parsePermissionState(%q, %q) = %q, want %q", tc.role, tc.value, got, tc.want)
		}
	}
}

func TestValidatePermissionSetFlags(t *testing.T) {
	if err := validatePermissionSetFlags("alice", "approved", ""); err != nil {
		t.Errorf("admin-only set should be valid: %v", err)
	}
	if err := validatePermissionSetFlags("alice", "", "request"); err != nil {
		t.Errorf("builder-only set should be valid: %v", err)
	}
	if err := validatePermissionSetFlags("", "approved", ""); err == nil {
		t.Error("missing --user should be invalid")
	}
	if err := validatePermissionSetFlags("alice", "", ""); err == nil {
		t.Error("no roles should be invalid")
	}
}

func TestValidatePermissionRequestFlags(t *testing.T) {
	if err := validatePermissionRequestFlags(true, false); err != nil {
		t.Errorf("admin request should be valid: %v", err)
	}
	if err := validatePermissionRequestFlags(false, true); err != nil {
		t.Errorf("builder request should be valid: %v", err)
	}
	if err := validatePermissionRequestFlags(false, false); err == nil {
		t.Error("no roles should be invalid")
	}
}
