package cli

import (
	"reflect"
	"testing"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/ref"
)

func TestParseBuildRef(t *testing.T) {
	r, err := parseBuildRef([]string{"123"})
	if err != nil {
		t.Fatalf("parseBuildRef: %v", err)
	}
	if r.Kind != ref.KindBuild || r.BuildID != 123 {
		t.Errorf("parseBuildRef = kind %d build id %d, want KindBuild 123", r.Kind, r.BuildID)
	}
}

func TestParseBuildRefRejectsNonBuild(t *testing.T) {
	for _, in := range []string{"owner/proj", "owner/proj/pkg", "123/fedora-40-x86_64"} {
		_, err := parseBuildRef([]string{in})
		if err == nil {
			t.Errorf("parseBuildRef(%q) expected an error", in)
			continue
		}
		if cerr.ExitCodeFor(err) != cerr.ExitUsage {
			t.Errorf("parseBuildRef(%q) exit code = %d, want %d", in, cerr.ExitCodeFor(err), cerr.ExitUsage)
		}
	}
}

func TestParseBuildChrootRef(t *testing.T) {
	r, err := parseBuildChrootRef([]string{"123/fedora-40-x86_64"})
	if err != nil {
		t.Fatalf("parseBuildChrootRef: %v", err)
	}
	if r.Kind != ref.KindBuildChroot || r.BuildID != 123 || r.BuildCht != "fedora-40-x86_64" {
		t.Errorf("parseBuildChrootRef = kind %d build %d chroot %q, want KindBuildChroot 123 fedora-40-x86_64", r.Kind, r.BuildID, r.BuildCht)
	}
}

func TestParseBuildChrootRefRejectsBuild(t *testing.T) {
	_, err := parseBuildChrootRef([]string{"123"})
	if err == nil {
		t.Fatal("parseBuildChrootRef expected an error for a plain build id")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitUsage)
	}
}

func TestParseBuildID(t *testing.T) {
	id, err := parseBuildID([]string{"42"})
	if err != nil {
		t.Fatalf("parseBuildID: %v", err)
	}
	if id != 42 {
		t.Errorf("parseBuildID = %d, want 42", id)
	}
	if _, err := parseBuildID([]string{"owner/proj"}); cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("parseBuildID(owner/proj) exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitUsage)
	}
}

func TestParseBuildIDs(t *testing.T) {
	ids, err := parseBuildIDs([]string{"1", "2", "3"})
	if err != nil {
		t.Fatalf("parseBuildIDs: %v", err)
	}
	if !reflect.DeepEqual(ids, []int{1, 2, 3}) {
		t.Errorf("parseBuildIDs = %v, want [1 2 3]", ids)
	}
	if _, err := parseBuildIDs([]string{"1", "owner/proj"}); err == nil {
		t.Error("parseBuildIDs expected an error for a non-build ref")
	}
}

func TestParsePackageRef(t *testing.T) {
	r, err := parsePackageRef([]string{"owner/proj/epel-9-x86_64"})
	if err != nil {
		t.Fatalf("parsePackageRef: %v", err)
	}
	if r.Kind != ref.KindPackage || r.Segment != "epel-9-x86_64" {
		t.Errorf("parsePackageRef = kind %d segment %q, want KindPackage epel-9-x86_64", r.Kind, r.Segment)
	}
}

func TestParseRefRequiresOwner(t *testing.T) {
	_, err := parseRef("mypkg")
	if err == nil {
		t.Fatal("parseRef expected an error for a bare reference")
	}
	if cerr.ExitCodeFor(err) != cerr.ExitUsage {
		t.Errorf("exit code = %d, want %d", cerr.ExitCodeFor(err), cerr.ExitUsage)
	}
}
