// Package rpmfile reads RPM package metadata in pure Go. It parses only the
// lead and header regions of the file; the compressed payload is never
// unpacked.
package rpmfile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/abn/coprctl/internal/cerr"
)

// RPM is the metadata of one RPM package, read from its main header.
type RPM struct {
	Name    string
	Version string
	Release string
	Arch    string
	Summary string
	Files   []FileInfo
}

// FileInfo is one file the package installs.
type FileInfo struct {
	Name string      // full path, e.g. /usr/bin/coprctl
	Mode os.FileMode // file type and permission bits
	Size int64       // unpacked size in bytes
}

// RPM lead and header layout. The lead is a fixed 96 bytes; each header is an
// 8-byte magic, index count and data length, an index of 16-byte entries, and
// the data they point into.
const (
	leadSize    = 96
	leadMagic   = 0xedabeedb
	headerMagic = "\x8e\xad\xe8\x01\x00\x00\x00\x00"
)

// Main header tag ids.
const (
	tagName           = 1000
	tagVersion        = 1001
	tagRelease        = 1002
	tagSummary        = 1004
	tagArch           = 1022
	tagOldFilenames   = 1027
	tagFileSizes      = 1028
	tagFileModes      = 1030
	tagFileDirIndexes = 1116
	tagBaseNames      = 1117
	tagDirNames       = 1118
	tagFileNames      = 5000
)

// Tag type codes. The classic v3 format and rpm 6 number the types
// differently: classic INT16/INT32/STRING/STRING_ARRAY are 9/3/4/6, rpm 6
// INT16/INT32/STRING/STRING_ARRAY are 3/4/6/8. String tags are recognized by
// value rather than by relying on a single numbering.
const (
	typeClassicInt16       = 9
	typeClassicInt32       = 3
	typeRPM6Int32          = 4
	typeClassicString      = 4
	typeClassicStringArray = 6
	typeRPM6String         = 6
	typeRPM6StringArray    = 8
)

// entry is one header index entry.
type entry struct {
	tag    uint32
	typ    uint32
	offset uint32
	count  uint32
}

// header is a parsed header region: its index entries and the data section
// they reference.
type header struct {
	entries []entry
	data    []byte
}

// Open parses the RPM file at path and returns its metadata.
func Open(path string) (*RPM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, cerr.Config("cannot read rpm").Wrap(err)
	}
	if len(data) < leadSize || binary.BigEndian.Uint32(data) != leadMagic {
		return nil, cerr.Config(fmt.Sprintf("%s is not an RPM (bad magic)", path))
	}
	off := align8(leadSize)
	if hasMagic(data, off) {
		first, next, err := parseHeader(data, off)
		if err != nil {
			return nil, err
		}
		if isSignature(first) || hasMagic(data, next) {
			off = next
		} else {
			// No signature header; the first header is the main header.
			return parseMain(first)
		}
	}
	if !hasMagic(data, off) {
		return nil, cerr.Config(fmt.Sprintf("%s has no main header", path))
	}
	main, _, err := parseHeader(data, off)
	if err != nil {
		return nil, err
	}
	return parseMain(main)
}

// isSignature reports whether the header is a signature header, which carries
// the digest tags 267-273 that no main header has.
func isSignature(h *header) bool {
	for _, e := range h.entries {
		if e.tag >= 267 && e.tag <= 273 {
			return true
		}
	}
	return false
}

func hasMagic(data []byte, off int) bool {
	return off+8 <= len(data) && string(data[off:off+8]) == headerMagic
}

func align8(n int) int { return (n + 7) &^ 7 }

// parseHeader reads one header region starting at off and returns it plus the
// 8-byte-aligned offset of the next region.
func parseHeader(data []byte, off int) (*header, int, error) {
	if off+16 > len(data) {
		return nil, 0, cerr.Config("rpm header is truncated")
	}
	if string(data[off:off+8]) != headerMagic {
		return nil, 0, cerr.Config("bad rpm header magic")
	}
	il := int(binary.BigEndian.Uint32(data[off+8:]))
	dl := int(binary.BigEndian.Uint32(data[off+12:]))
	indexOff := off + 16
	if indexOff+il*16 > len(data) {
		return nil, 0, cerr.Config("rpm header index is truncated")
	}
	h := &header{entries: make([]entry, 0, il)}
	for i := 0; i < il; i++ {
		e := indexOff + i*16
		h.entries = append(h.entries, entry{
			tag:    binary.BigEndian.Uint32(data[e:]),
			typ:    binary.BigEndian.Uint32(data[e+4:]),
			offset: binary.BigEndian.Uint32(data[e+8:]),
			count:  binary.BigEndian.Uint32(data[e+12:]),
		})
	}
	start := indexOff + il*16
	if start+dl > len(data) {
		return nil, 0, cerr.Config("rpm header data is truncated")
	}
	h.data = data[start : start+dl]
	return h, align8(start + dl), nil
}

func parseMain(h *header) (*RPM, error) {
	r := &RPM{
		Name:    h.str(tagName),
		Version: h.str(tagVersion),
		Release: h.str(tagRelease),
		Arch:    h.str(tagArch),
		Summary: h.str(tagSummary),
	}
	files, err := h.files()
	if err != nil {
		return nil, err
	}
	r.Files = files
	return r, nil
}

// str returns the first string value of tag, or "".
func (h *header) str(tag uint32) string {
	e, ok := h.find(tag)
	if !ok {
		return ""
	}
	s, err := h.strings(e)
	if err != nil || len(s) == 0 {
		return ""
	}
	return s[0]
}

// files reconstructs the installed file list from the header tags.
func (h *header) files() ([]FileInfo, error) {
	names, err := h.fileNames()
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, nil
	}
	sizes := make([]int64, len(names))
	if e, ok := h.find(tagFileSizes); ok {
		if v, err := h.ints(e, 4); err == nil {
			for i := range sizes {
				if i < len(v) {
					sizes[i] = v[i]
				}
			}
		}
	}
	modes := make([]os.FileMode, len(names))
	if e, ok := h.find(tagFileModes); ok {
		if v, err := h.modes(e); err == nil {
			for i := range modes {
				if i < len(v) {
					modes[i] = modeToFileMode(v[i])
				}
			}
		}
	}
	files := make([]FileInfo, len(names))
	for i, n := range names {
		files[i] = FileInfo{Name: n, Mode: modes[i], Size: sizes[i]}
	}
	return files, nil
}

// fileNames resolves the full paths of all files in the package. Modern
// packages use FILENAMES; some use the older OLDFILENAMES; and compressed-file
// packages split paths into BASENAMES, DIRNAMES, and a per-file directory
// index.
func (h *header) fileNames() ([]string, error) {
	if e, ok := h.find(tagFileNames); ok {
		return h.strings(e)
	}
	if e, ok := h.find(tagOldFilenames); ok {
		return h.strings(e)
	}
	be, ok := h.find(tagBaseNames)
	if !ok {
		return nil, nil
	}
	dn, ok := h.find(tagDirNames)
	ix, ok3 := h.find(tagFileDirIndexes)
	if !ok || !ok3 {
		return nil, nil
	}
	base, err := h.strings(be)
	if err != nil {
		return nil, err
	}
	dirs, err := h.strings(dn)
	if err != nil {
		return nil, err
	}
	idxs, err := h.ints(ix, 4)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(base))
	for i, b := range base {
		if i >= len(idxs) {
			return nil, cerr.Config("rpm file list is inconsistent")
		}
		d := idxs[i]
		if d < 0 || d >= int64(len(dirs)) {
			return nil, cerr.Config("rpm file list has an out-of-range directory index")
		}
		names[i] = dirs[d] + b
	}
	return names, nil
}

// find returns the entry for tag.
func (h *header) find(tag uint32) (entry, bool) {
	for _, e := range h.entries {
		if e.tag == tag {
			return e, true
		}
	}
	return entry{}, false
}

// strings reads count NUL-terminated strings from the entry's data.
func (h *header) strings(e entry) ([]string, error) {
	base := int(e.offset)
	out := make([]string, 0, e.count)
	for i := 0; i < int(e.count); i++ {
		if base >= len(h.data) {
			return nil, cerr.Config("rpm header string data is truncated")
		}
		end := bytes.IndexByte(h.data[base:], 0)
		if end < 0 {
			return nil, cerr.Config("rpm header contains an unterminated string")
		}
		out = append(out, string(h.data[base:base+end]))
		base += end + 1
	}
	return out, nil
}

// ints reads count big-endian integers of the given element size (2 or 4).
func (h *header) ints(e entry, size int) ([]int64, error) {
	n := int(e.count)
	base := int(e.offset)
	if base+n*size > len(h.data) {
		return nil, cerr.Config("rpm header integer data is truncated")
	}
	out := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		if size == 2 {
			out = append(out, int64(binary.BigEndian.Uint16(h.data[base+i*2:])))
		} else {
			out = append(out, int64(int32(binary.BigEndian.Uint32(h.data[base+i*4:]))))
		}
	}
	return out, nil
}

// modes reads the raw mode values of every file, resolving the classic and
// rpm 6 tag type numberings.
func (h *header) modes(e entry) ([]uint32, error) {
	n := int(e.count)
	base := int(e.offset)
	if n == 0 {
		return nil, nil
	}
	sizes := []int{2, 4}
	switch e.typ {
	case typeClassicInt16:
		sizes = []int{2}
	case typeRPM6Int32:
		sizes = []int{4}
	case typeClassicInt32: // classic INT32 or rpm 6 INT16
		switch h.scheme() {
		case schemeNew:
			sizes = []int{2}
		default:
			sizes = []int{4}
		}
	}
	var best []uint32
	for _, size := range sizes {
		if base+n*size > len(h.data) {
			continue
		}
		vals := make([]uint32, 0, n)
		for i := 0; i < n; i++ {
			if size == 2 {
				vals = append(vals, uint32(binary.BigEndian.Uint16(h.data[base+i*2:])))
			} else {
				vals = append(vals, binary.BigEndian.Uint32(h.data[base+i*4:]))
			}
		}
		if plausibleModes(vals) {
			return vals, nil
		}
		if best == nil {
			best = vals
		}
	}
	if best != nil {
		return best, nil
	}
	return nil, cerr.Config("rpm header file modes are truncated")
}

// Header type-numbering schemes.
const (
	schemeClassic = iota
	schemeNew
)

// scheme detects the tag type numbering the header uses. rpm 6 renumbered the
// types; the string tags distinguish the two.
func (h *header) scheme() int {
	for _, tag := range []uint32{tagName, tagVersion, tagRelease, tagArch} {
		if e, ok := h.find(tag); ok {
			switch e.typ {
			case typeClassicString:
				return schemeClassic
			case typeRPM6String:
				return schemeNew
			}
		}
	}
	for _, tag := range []uint32{tagFileNames, tagOldFilenames, tagBaseNames, tagDirNames} {
		if e, ok := h.find(tag); ok {
			switch e.typ {
			case typeClassicStringArray:
				return schemeClassic
			case typeRPM6StringArray:
				return schemeNew
			}
		}
	}
	return schemeClassic
}

// plausibleModes reports whether every value looks like a Unix mode: file-type
// bits in the S_IFMT range and permission bits within 07777.
func plausibleModes(vals []uint32) bool {
	for _, v := range vals {
		if v > 0o177777 {
			return false
		}
		switch v >> 12 {
		case 0, 1, 2, 4, 6, 8, 10, 12: // plain, fifo, chr, dir, blk, reg, lnk, sock
		default:
			return false
		}
	}
	return true
}

// Unix file type bits, mirroring syscall.S_IF* without the platform dependency.
const (
	fileTypeMask = 0o170000
	fileTypeFifo = 0o010000
	fileTypeChr  = 0o020000
	fileTypeDir  = 0o040000
	fileTypeBlk  = 0o060000
	fileTypeReg  = 0o100000
	fileTypeLnk  = 0o120000
	fileTypeSock = 0o140000
)

// modeToFileMode converts a Unix mode_t to an os.FileMode.
func modeToFileMode(m uint32) os.FileMode {
	var fm os.FileMode
	switch m & fileTypeMask {
	case fileTypeDir:
		fm |= os.ModeDir
	case fileTypeLnk:
		fm |= os.ModeSymlink
	case fileTypeChr:
		fm |= os.ModeDevice | os.ModeCharDevice
	case fileTypeBlk:
		fm |= os.ModeDevice
	case fileTypeFifo:
		fm |= os.ModeNamedPipe
	case fileTypeSock:
		fm |= os.ModeSocket
	}
	if m&0o4000 != 0 {
		fm |= os.ModeSetuid
	}
	if m&0o2000 != 0 {
		fm |= os.ModeSetgid
	}
	if m&0o1000 != 0 {
		fm |= os.ModeSticky
	}
	return fm | os.FileMode(m&0o777)
}
