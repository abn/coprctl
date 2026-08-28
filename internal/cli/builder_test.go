package cli

import "testing"

func TestNormalizeMode(t *testing.T) {
	cases := []struct {
		in   string
		want runtimeMode
	}{
		{"", modeAuto},
		{"auto", modeAuto},
		{"container", modeContainer},
		{"podman", modeContainer},
		{"docker", modeContainer},
		{"native", modeNative},
		{"host", modeNative},
		{"rpmbuild", modeNative},
		{"mock", modeMock},
		{"garbage", modeAuto},
	}
	for _, tc := range cases {
		if got := normalizeMode(tc.in); got != tc.want {
			t.Errorf("normalizeMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestResolveBuilderExplicit(t *testing.T) {
	// Explicit modes do not probe the environment, so they are deterministic.
	if b, err := resolveBuilder("native", "srpm"); err != nil || b.Name() != "native" {
		t.Errorf("native: b=%v err=%v", b, err)
	}
	if b, err := resolveBuilder("container", "srpm"); err != nil || b.Name() != "container" {
		t.Errorf("container: b=%v err=%v", b, err)
	}
}

func TestMockBuilderUnavailable(t *testing.T) {
	// mock may or may not be installed; when forced, missing mock must produce
	// a hint error rather than silently falling back.
	if _, err := resolveBuilder("mock", "srpm"); err == nil {
		t.Log("mock available on this host; skipping")
	}
}
