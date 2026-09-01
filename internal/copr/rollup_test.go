package copr

import (
	"encoding/json"
	"testing"
)

func TestAttachBuildChroots(t *testing.T) {
	var l struct {
		Items []BuildChroot `json:"items"`
	}
	if err := json.Unmarshal(readFixture(t, "testdata/build-chroot-list.json"), &l); err != nil {
		t.Fatalf("decode: %v", err)
	}
	b := &Build{}
	b.AttachBuildChroots(l.Items)
	states := b.ChrootStates()
	if states["epel-9-x86_64"] != "failed" {
		t.Errorf("ChrootStates()[epel-9-x86_64] = %q, want failed", states["epel-9-x86_64"])
	}
	if states["fedora-rawhide-x86_64"] != "succeeded" {
		t.Errorf("ChrootStates()[fedora-rawhide-x86_64] = %q, want succeeded", states["fedora-rawhide-x86_64"])
	}
	if got := b.RollupState(); got != "failed" {
		t.Errorf("RollupState() = %q, want failed", got)
	}
}

func TestRollupState(t *testing.T) {
	tests := []struct {
		name   string
		build  *Build
		chroot string
		state  string
		want   string
	}{
		{"all succeeded", &Build{}, "b", "succeeded", "succeeded"},
		{"any failed", &Build{Chroots: []string{"a", "b"}}, "b", "failed", "failed"},
		{"any canceled", &Build{Chroots: []string{"a", "b"}}, "b", "canceled", "canceled"},
		{"any running", &Build{Chroots: []string{"a", "b"}}, "b", "running", "running"},
		{"any pending", &Build{Chroots: []string{"a", "b"}}, "b", "importing", "pending"},
		{"failed beats canceled", &Build{Chroots: []string{"a", "b"}}, "b", "failed", "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a state map via Builds for precise per-chroot control.
			if tt.chroot != "" {
				tt.build.Builds = map[string]*BuildChroot{
					"a": {State: "succeeded"},
					"b": {State: tt.state},
				}
			}
			if got := tt.build.RollupState(); got != tt.want {
				t.Errorf("RollupState() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChrootStatesFallback(t *testing.T) {
	b := &Build{State: "running", Chroots: []string{"x", "y"}}
	m := b.ChrootStates()
	if len(m) != 2 || m["x"] != "running" || m["y"] != "running" {
		t.Errorf("ChrootStates() = %v", m)
	}
}

func TestTerminalRunning(t *testing.T) {
	if !IsTerminal("succeeded") || !IsTerminal("failed") || IsTerminal("running") {
		t.Errorf("terminal state detection wrong")
	}
	if !IsRunning("running") || !IsRunning("starting") || IsRunning("pending") {
		t.Errorf("running state detection wrong")
	}
}
