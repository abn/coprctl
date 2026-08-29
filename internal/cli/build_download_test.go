package cli

import (
	"reflect"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestFilterBuildChroots(t *testing.T) {
	bp := copr.BuiltPackages{
		"fedora-rawhide-x86_64": {},
		"fedora-42-x86_64":      {},
		"fedora-42-aarch64":     {},
		"epel-9-x86_64":         {},
	}
	tests := []struct {
		name  string
		globs []string
		want  []string
	}{
		{name: "no globs keeps all", want: []string{"epel-9-x86_64", "fedora-42-aarch64", "fedora-42-x86_64", "fedora-rawhide-x86_64"}},
		{name: "single glob", globs: []string{"epel-*"}, want: []string{"epel-9-x86_64"}},
		{name: "multiple globs", globs: []string{"fedora-42-*", "epel-*"}, want: []string{"epel-9-x86_64", "fedora-42-aarch64", "fedora-42-x86_64"}},
		{name: "question mark", globs: []string{"fedora-4?-x86_64"}, want: []string{"fedora-42-x86_64"}},
		{name: "no match", globs: []string{"rhel-*"}, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterBuildChroots(bp, tt.globs)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterBuildChroots = %v, want %v", got, tt.want)
			}
		})
	}
}
