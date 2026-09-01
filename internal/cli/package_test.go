package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
)

func TestPackageCreateRejectsRpmUpload(t *testing.T) {
	app := NewApp()
	cmd := newPackageCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"create", "owner/proj/pkg", "--source", "rpm-upload"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a usage error for rpm-upload source")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("exit code = %d, want usage", cerr.ExitCodeFor(err))
	}
	if !strings.Contains(err.Error(), "build submit") {
		t.Errorf("error = %q, want it to point at build submit", err)
	}
}

func TestPackageEditRejectsRpmUpload(t *testing.T) {
	app := NewApp()
	cmd := newPackageCmd(app)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"edit", "owner/proj/pkg", "--source", "rpm-upload"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a usage error for rpm-upload source")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("exit code = %d, want usage", cerr.ExitCodeFor(err))
	}
}
