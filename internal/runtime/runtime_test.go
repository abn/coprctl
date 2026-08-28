package runtime

import (
	"context"
	"strings"
	"testing"
)

// fakeRuntime records invocations for testing.
type fakeRuntime struct {
	runs   []string
	builds []string
	root   bool
}

func (f *fakeRuntime) Name() string     { return "fake" }
func (f *fakeRuntime) Rootless() bool   { return f.root }
func (f *fakeRuntime) Available() error { return nil }
func (f *fakeRuntime) Run(_ context.Context, spec RunSpec) error {
	f.runs = append(f.runs, strings.Join(spec.Args, " "))
	return nil
}
func (f *fakeRuntime) Build(_ context.Context, spec BuildSpec) error {
	f.builds = append(f.builds, spec.Tag)
	return nil
}

func TestDetect(t *testing.T) {
	// Since podman is likely present, Detect should return something.
	r, err := Detect("")
	if err == nil {
		if r.Name() != "podman" && r.Name() != "docker" {
			t.Errorf("unexpected runtime %q", r.Name())
		}
	}
}

func TestFakeRuntimeRun(t *testing.T) {
	f := &fakeRuntime{}
	if err := f.Run(context.Background(), RunSpec{Image: "quay.io/x/rpmbuilder:fedora-44", Args: []string{"rpmbuild", "-ba"}}); err != nil {
		t.Fatal(err)
	}
	if len(f.runs) != 1 || f.runs[0] != "rpmbuild -ba" {
		t.Errorf("runs = %v", f.runs)
	}
}

func TestMountArg(t *testing.T) {
	got := mountArgAt("/tmp/work", "/sources")
	// Host prefix is OS-dependent; assert the container mount tail with the
	// optional Linux SELinux relabel.
	if !strings.HasSuffix(got, ":/sources") && !strings.HasSuffix(got, ":/sources:z") {
		t.Errorf("mountArgAt(/tmp/work, /sources) = %q, want container mount tail", got)
	}
}
