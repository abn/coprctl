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
