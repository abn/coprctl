package rpmfile

import (
	"bytes"
	"encoding/binary"
	"os"
	"path"
	"path/filepath"
	"testing"
)

// fixtureFile is one file the fixture RPM claims to install.
type fixtureFile struct {
	name string
	mode uint16
	size int64
}

// fixtureTag is one main-header index entry plus its raw data.
type fixtureTag struct {
	tag   uint32
	typ   uint32
	count uint32
	data  []byte
}

// Header type codes used to build fixtures.
const (
	fixtureStrT  = 4 // classic STRING, rpm 6 INT32
	fixtureArrT  = 6 // classic STRING_ARRAY, rpm 6 STRING
	fixtureI16T  = 9 // classic INT16
	fixtureI32T  = 3 // classic INT32, rpm 6 INT16
	fixtureStrT6 = 6 // rpm 6 STRING
	fixtureArrT8 = 8 // rpm 6 STRING_ARRAY
	fixtureI16T6 = 3 // rpm 6 INT16
	fixtureI32T6 = 4 // rpm 6 INT32
)

func fixtureAlign(typ uint32) int {
	switch typ {
	case fixtureI32T: // classic INT32 / rpm 6 INT16
		return 4
	case fixtureI32T6: // rpm 6 INT32
		return 4
	case fixtureI16T: // classic INT16
		return 2
	default:
		return 1
	}
}

// buildHeader serializes a header region from its index entries and data.
func buildHeader(tags []fixtureTag) []byte {
	var data []byte
	entries := make([]entry, len(tags))
	for i, t := range tags {
		for len(data)%fixtureAlign(t.typ) != 0 {
			data = append(data, 0)
		}
		off := len(data)
		data = append(data, t.data...)
		entries[i] = entry{tag: t.tag, typ: t.typ, offset: uint32(off), count: t.count}
	}
	var buf bytes.Buffer
	buf.WriteString(headerMagic)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(tags)))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(data)))
	for _, e := range entries {
		_ = binary.Write(&buf, binary.BigEndian, e.tag)
		_ = binary.Write(&buf, binary.BigEndian, e.typ)
		_ = binary.Write(&buf, binary.BigEndian, e.offset)
		_ = binary.Write(&buf, binary.BigEndian, e.count)
	}
	buf.Write(data)
	return buf.Bytes()
}

// buildRPM assembles a full fixture: lead, a minimal signature header, and the
// given main header.
func buildRPM(t *testing.T, main []byte) string {
	t.Helper()
	lead := make([]byte, leadSize)
	binary.BigEndian.PutUint32(lead, leadMagic)
	var buf bytes.Buffer
	buf.Write(lead)
	buf.Write(buildHeader(nil))
	buf.Write(main)
	path := filepath.Join(t.TempDir(), "fixture.rpm")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func strData(ss ...string) []byte {
	var b bytes.Buffer
	for _, s := range ss {
		b.WriteString(s)
		b.WriteByte(0)
	}
	return b.Bytes()
}

func int16Data(vals ...uint16) []byte {
	var b bytes.Buffer
	for _, v := range vals {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	return b.Bytes()
}

func int32Data(vals ...uint32) []byte {
	var b bytes.Buffer
	for _, v := range vals {
		_ = binary.Write(&b, binary.BigEndian, v)
	}
	return b.Bytes()
}

// metaTags builds the NAME/VERSION/RELEASE/ARCH/SUMMARY entries.
func metaTags(strT uint32, name, version, release, arch, summary string) []fixtureTag {
	return []fixtureTag{
		{tagName, strT, 1, strData(name)},
		{tagVersion, strT, 1, strData(version)},
		{tagRelease, strT, 1, strData(release)},
		{tagArch, strT, 1, strData(arch)},
		{tagSummary, strT, 1, strData(summary)},
	}
}

// fileTags builds the file-list entries. kind selects the on-disk layout:
// "filenames", "oldfilenames", or "split" (BASENAMES + DIRNAMES + index).
func fileTags(arrT, i16T, i32T uint32, files []fixtureFile, kind string) []fixtureTag {
	names := make([]string, len(files))
	modes := make([]uint16, len(files))
	sizes := make([]uint32, len(files))
	for i, f := range files {
		names[i] = f.name
		modes[i] = f.mode
		sizes[i] = uint32(f.size)
	}
	var tags []fixtureTag
	switch kind {
	case "filenames":
		tags = append(tags, fixtureTag{tagFileNames, arrT, uint32(len(files)), strData(names...)})
	case "oldfilenames":
		tags = append(tags, fixtureTag{tagOldFilenames, arrT, uint32(len(files)), strData(names...)})
	case "split":
		dirIdx := map[string]uint32{}
		var dirs []string
		bases := make([]string, len(files))
		idxs := make([]uint32, len(files))
		for i, f := range files {
			dir, base := path.Split(f.name)
			j, ok := dirIdx[dir]
			if !ok {
				j = uint32(len(dirs))
				dirIdx[dir] = j
				dirs = append(dirs, dir)
			}
			bases[i] = base
			idxs[i] = j
		}
		tags = append(tags, fixtureTag{tagBaseNames, arrT, uint32(len(bases)), strData(bases...)})
		tags = append(tags, fixtureTag{tagDirNames, arrT, uint32(len(dirs)), strData(dirs...)})
		tags = append(tags, fixtureTag{tagFileDirIndexes, i32T, uint32(len(idxs)), int32Data(idxs...)})
	}
	tags = append(tags,
		fixtureTag{tagFileSizes, i32T, uint32(len(files)), int32Data(sizes...)},
		fixtureTag{tagFileModes, i16T, uint32(len(files)), int16Data(modes...)},
	)
	return tags
}

func TestOpenParsesMetadataAndFiles(t *testing.T) {
	files := []fixtureFile{
		{name: "/usr/bin/coprctl", mode: 0o100755, size: 8936440},
		{name: "/usr/share/doc/coprctl/README.md", mode: 0o100644, size: 1209},
	}
	// Classic and rpm 6 type numberings both occur in the wild.
	for _, tc := range []struct {
		name     string
		strT     uint32
		arrT     uint32
		i16T     uint32
		i32T     uint32
		fileKind string
	}{
		{"classic", fixtureStrT, fixtureArrT, fixtureI16T, fixtureI32T, "filenames"},
		{"rpm6", fixtureStrT6, fixtureArrT8, fixtureI16T6, fixtureI32T6, "filenames"},
	} {
		tags := metaTags(tc.strT, "coprctl", "0.5.0", "1.fc44", "x86_64", "a CLI")
		tags = append(tags, fileTags(tc.arrT, tc.i16T, tc.i32T, files, tc.fileKind)...)
		r, err := Open(buildRPM(t, buildHeader(tags)))
		if err != nil {
			t.Fatalf("%s: Open: %v", tc.name, err)
		}
		if r.Name != "coprctl" || r.Version != "0.5.0" || r.Release != "1.fc44" ||
			r.Arch != "x86_64" || r.Summary != "a CLI" {
			t.Fatalf("%s: metadata = %+v", tc.name, r)
		}
		if len(r.Files) != 2 {
			t.Fatalf("%s: files = %+v", tc.name, r.Files)
		}
		first := r.Files[0]
		if first.Name != "/usr/bin/coprctl" || first.Size != 8936440 || first.Mode.String() != "-rwxr-xr-x" {
			t.Fatalf("%s: first file = %+v", tc.name, first)
		}
		if got := r.Files[1].Mode.String(); got != "-rw-r--r--" {
			t.Fatalf("%s: second file mode = %q", tc.name, got)
		}
	}
}

func TestOpenDirectoryAndSymlinkModes(t *testing.T) {
	files := []fixtureFile{
		{name: "/usr/share/coprctl/", mode: 0o040755, size: 0},
		{name: "/usr/bin/coprctl", mode: 0o120777, size: 5},
	}
	tags := metaTags(fixtureStrT6, "coprctl", "0.5.0", "1.fc44", "x86_64", "")
	tags = append(tags, fileTags(fixtureArrT8, fixtureI16T6, fixtureI32T6, files, "filenames")...)
	r, err := Open(buildRPM(t, buildHeader(tags)))
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Files[0].Mode.String(); got != "drwxr-xr-x" {
		t.Errorf("dir mode = %q, want drwxr-xr-x", got)
	}
	if got := r.Files[1].Mode.String(); got != "Lrwxrwxrwx" {
		t.Errorf("link mode = %q, want Lrwxrwxrwx", got)
	}
}

func TestOpenFallsBackToOldFilenames(t *testing.T) {
	files := []fixtureFile{{name: "/usr/bin/old", mode: 0o100755, size: 7}}
	tags := metaTags(fixtureStrT, "pkg", "1", "1", "x86_64", "")
	tags = append(tags, fileTags(fixtureArrT, fixtureI16T, fixtureI32T, files, "oldfilenames")...)
	r, err := Open(buildRPM(t, buildHeader(tags)))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 1 || r.Files[0].Name != "/usr/bin/old" {
		t.Fatalf("files = %+v", r.Files)
	}
}

func TestOpenFallsBackToSplitNames(t *testing.T) {
	files := []fixtureFile{
		{name: "/usr/bin/coprctl", mode: 0o100755, size: 8936440},
		{name: "/usr/share/coprctl/data", mode: 0o100644, size: 42},
	}
	tags := metaTags(fixtureStrT6, "coprctl", "0.5.0", "1.fc44", "x86_64", "")
	tags = append(tags, fileTags(fixtureArrT8, fixtureI16T6, fixtureI32T6, files, "split")...)
	r, err := Open(buildRPM(t, buildHeader(tags)))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 2 {
		t.Fatalf("files = %+v", r.Files)
	}
	if r.Files[0].Name != "/usr/bin/coprctl" || r.Files[1].Name != "/usr/share/coprctl/data" {
		t.Fatalf("files = %+v", r.Files)
	}
	if r.Files[0].Size != 8936440 || r.Files[0].Mode.String() != "-rwxr-xr-x" {
		t.Fatalf("first file = %+v", r.Files[0])
	}
}

func TestOpenNoFileList(t *testing.T) {
	tags := metaTags(fixtureStrT6, "pkg", "1", "1", "x86_64", "")
	r, err := Open(buildRPM(t, buildHeader(tags)))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 0 {
		t.Fatalf("expected no files, got %+v", r.Files)
	}
}

func TestOpenRejectsNonRPM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not.rpm")
	if err := os.WriteFile(path, []byte("this is not an rpm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("expected an error for a non-RPM file")
	}
}

func TestOpenMissingFile(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "nope.rpm")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

func TestOpenNoSignatureHeader(t *testing.T) {
	files := []fixtureFile{{name: "/usr/bin/coprctl", mode: 0o100755, size: 8936440}}
	tags := metaTags(fixtureStrT, "coprctl", "0.5.0", "1.fc44", "x86_64", "")
	tags = append(tags, fileTags(fixtureArrT, fixtureI16T, fixtureI32T, files, "filenames")...)
	lead := make([]byte, leadSize)
	binary.BigEndian.PutUint32(lead, leadMagic)
	var buf bytes.Buffer
	buf.Write(lead)
	buf.Write(buildHeader(tags))
	path := filepath.Join(t.TempDir(), "nosig.rpm")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Files) != 1 || r.Files[0].Name != "/usr/bin/coprctl" {
		t.Fatalf("files = %+v", r.Files)
	}
}

func TestOpenTruncatedHeader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short.rpm")
	lead := make([]byte, leadSize)
	binary.BigEndian.PutUint32(lead, leadMagic)
	// Signature header declares data that is not present.
	lead = append(lead, []byte("\x8e\xad\xe8\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x10\x00")...)
	if err := os.WriteFile(path, lead, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); err == nil {
		t.Fatal("expected an error for a truncated header")
	}
}
