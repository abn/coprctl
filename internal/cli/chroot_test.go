package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/chroot"
	"github.com/abn/coprctl/internal/copr"
)

func TestWarnIfInactive(t *testing.T) {
	var buf bytes.Buffer
	cmd := newChrootCmd(NewApp())
	cmd.SetErr(&buf)

	// EOL chroot in catalog: warns.
	catalog := copr.MockChroots{
		"fedora-42-x86_64": "Fedora 42",
		"fedora-43-x86_64": "Fedora 43",
	}
	cmd.SetArgs(nil)
	warnIfInactive(cmd, "fedora-42-x86_64", catalog)
	if !strings.Contains(buf.String(), "preserved") {
		t.Errorf("expected preserved warning, got %q", buf.String())
	}

	// Active chroot: no warning.
	buf.Reset()
	warnIfInactive(cmd, "fedora-43-x86_64", catalog)
	if buf.Len() != 0 {
		t.Errorf("expected no warning for active chroot, got %q", buf.String())
	}
}

func TestFilterByState(t *testing.T) {
	states := []chroot.Info{
		{Name: "a", State: chroot.Active},
		{Name: "b", State: chroot.Preserved},
	}
	got := filterByState(states, "preserved")
	if len(got) != 1 || got[0].Name != "b" {
		t.Errorf("filterByState = %+v", got)
	}
}
