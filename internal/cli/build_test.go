package cli

import (
	"reflect"
	"testing"

	"github.com/abn/coprctl/internal/copr"
)

func TestFailedChroots(t *testing.T) {
	tests := []struct {
		name  string
		build *copr.Build
		want  []string
	}{
		{
			name: "detailed builds map",
			build: &copr.Build{
				Builds: map[string]*copr.BuildChroot{
					"fedora-rawhide-x86_64": {State: "failed"},
					"fedora-41-x86_64":      {State: "succeeded"},
					"epel-9-x86_64":         {State: "failed"},
					"fedora-42-x86_64":      {State: "running"},
				},
			},
			want: []string{"epel-9-x86_64", "fedora-rawhide-x86_64"},
		},
		{
			name: "chroots list fallback",
			build: &copr.Build{
				Chroots: []string{"fedora-rawhide-x86_64", "epel-9-x86_64"},
				State:   "failed",
			},
			want: []string{"epel-9-x86_64", "fedora-rawhide-x86_64"},
		},
		{
			name: "no failed chroots",
			build: &copr.Build{
				Builds: map[string]*copr.BuildChroot{
					"fedora-rawhide-x86_64": {State: "succeeded"},
					"epel-9-x86_64":         {State: "skipped"},
				},
			},
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := failedChroots(tt.build)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("failedChroots = %v, want %v", got, tt.want)
			}
		})
	}
}
