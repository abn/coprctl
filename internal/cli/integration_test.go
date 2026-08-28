package cli

import (
	"reflect"
	"testing"
)

func TestDefaultHookEvents(t *testing.T) {
	tests := []struct {
		name    string
		tagOnly bool
		events  string
		want    []string
	}{
		{"tag-only default", true, "", []string{"create"}},
		{"branch pushes opt-in", false, "", []string{"push", "create"}},
		{"explicit events override", true, "push", []string{"push"}},
		{"explicit events with tag-only true", true, "create,push", []string{"create", "push"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultHookEvents(tt.tagOnly, tt.events)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("defaultHookEvents(%v,%q) = %v, want %v", tt.tagOnly, tt.events, got, tt.want)
			}
		})
	}
}

func TestNormalizeCloneURL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://github.com/abn/coprctl.git", "github.com/abn/coprctl"},
		{"https://github.com/abn/coprctl", "github.com/abn/coprctl"},
		{"git@github.com:abn/coprctl.git", "github.com/abn/coprctl"},
		{"http://github.com/abn/coprctl", "github.com/abn/coprctl"},
	}
	for _, tc := range cases {
		if got := normalizeCloneURL(tc.in); got != tc.want {
			t.Errorf("normalizeCloneURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
