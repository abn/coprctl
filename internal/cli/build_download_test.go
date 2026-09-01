package cli

import (
	"reflect"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestSpecFilename(t *testing.T) {
	tests := []struct {
		name  string
		build copr.Build
		want  string
	}{
		{name: "package-scoped build", build: copr.Build{SourcePackage: copr.SourcePackage{Name: "coprctl"}}, want: "coprctl.spec"},
		{name: "project-scoped build falls back to project", build: copr.Build{ProjectName: "coprctl"}, want: "coprctl.spec"},
		{name: "both set prefers package", build: copr.Build{SourcePackage: copr.SourcePackage{Name: "pkg"}, ProjectName: "proj"}, want: "pkg.spec"},
		{name: "neither set yields bare spec", build: copr.Build{}, want: ".spec"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := specFilename(tt.build); got != tt.want {
				t.Errorf("specFilename(%+v) = %q, want %q", tt.build, got, tt.want)
			}
		})
	}
}

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
