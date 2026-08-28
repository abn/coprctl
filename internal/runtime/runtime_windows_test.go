package runtime

import (
	"strings"
	"testing"
)

// On Windows the host path must be slash-normalized so the container mount
// argument is well-formed for the runtime.
func TestMountArgWindows(t *testing.T) {
	got := mountArg(`C:\Users\me\work`)
	if !strings.HasSuffix(got, ":/work") {
		t.Errorf("mountArg = %q, want suffix :/work", got)
	}
	if strings.Contains(got, `\`) {
		t.Errorf("mountArg = %q, backslash host path not normalized", got)
	}
}
