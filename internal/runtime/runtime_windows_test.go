package runtime

import (
	"strings"
	"testing"
)

// On Windows the host path must be slash-normalized so the container mount
// argument is well-formed for the runtime.
func TestMountArgWindows(t *testing.T) {
	got := mountArgAt(`C:\Users\me\work`, "/sources")
	if !strings.HasSuffix(got, ":/sources") {
		t.Errorf("mountArgAt = %q, want suffix :/sources", got)
	}
	if strings.Contains(got, `\`) {
		t.Errorf("mountArgAt = %q, backslash host path not normalized", got)
	}
}
