// Package chroot implements the chroot catalog model: name parsing and the
// EOL (end-of-life) lifecycle states surfaced by chroot list. The Copr API
// returns mock chroots as a {name: comment} map with no machine-readable
// state, so lifecycle is derived from the name against a maintained EOL table.
package chroot

import (
	"fmt"
	"strings"
	"time"
)

// State is the lifecycle state of a chroot.
type State string

const (
	// Active means the distro release is current and accepts new builds.
	Active State = "active"
	// Preserved means the release is EOL: existing repos remain but no new
	// builds are accepted.
	Preserved State = "preserved"
	// Deleted means the chroot no longer exists in the instance catalog.
	Deleted State = "deleted"
)

// Info is a parsed chroot name plus its derived lifecycle state.
type Info struct {
	Name    string
	Distro  string
	Version string
	Arch    string
	State   State
}

// eol holds the EOL date (last day the release accepts new builds) for a
// distro/version pair. Dates are the final day a release is supported.
var eol = map[string]map[string]struct{ year, month, day int }{
	"fedora": {
		"32": {2021, 5, 25}, "33": {2021, 11, 30}, "34": {2022, 6, 7},
		"35": {2022, 12, 13}, "36": {2023, 5, 16}, "37": {2023, 12, 5},
		"38": {2024, 5, 21}, "39": {2024, 11, 26}, "40": {2025, 5, 13},
		"41": {2025, 12, 2}, "42": {2026, 6, 16}, "43": {2026, 12, 1},
	},
	"epel": {
		"7": {2024, 6, 30}, "8": {2029, 5, 31}, "9": {2032, 5, 31},
		"10": {2035, 5, 31},
	},
	"centos-stream": {
		"8": {2024, 5, 31}, "9": {2027, 5, 31}, "10": {2030, 5, 31},
	},
}

// evergreen lists release identifiers that never go EOL (rolling targets).
var evergreen = map[string]bool{
	"rawhide":  true,
	"branched": true,
}

// now is the time source; overridable in tests.
var now = func() time.Time { return time.Now() }

func today() time.Time {
	t := now()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func afterEOL(date struct{ year, month, day int }) bool {
	t := today()
	d := time.Date(date.year, time.Month(date.month), date.day, 0, 0, 0, 0, time.UTC)
	return t.After(d)
}

// Parse splits a chroot name into distro, version, and arch. A chroot name is
// <distro>-<version>-<arch>; the version is the first token that starts with a
// digit (so 15.6 stays one token), or a known named version such as rawhide.
// Everything before the version is the distro (e.g. "centos" + "stream").
func Parse(name string) (Info, error) {
	parts := strings.Split(name, "-")
	if len(parts) < 3 {
		return Info{}, fmt.Errorf("chroot name %q is not distro-version-arch", name)
	}
	arch := parts[len(parts)-1]
	versionIdx := -1
	for i, p := range parts[:len(parts)-1] {
		if startsWithDigit(p) || evergreen[p] {
			versionIdx = i
			break
		}
	}
	if versionIdx < 0 {
		return Info{}, fmt.Errorf("chroot name %q has no version", name)
	}
	distro := strings.Join(parts[:versionIdx], "-")
	version := parts[versionIdx]
	return Info{Name: name, Distro: distro, Version: version, Arch: arch}, nil
}

func startsWithDigit(s string) bool {
	return s != "" && s[0] >= '0' && s[0] <= '9'
}

// Classify determines the lifecycle state of a chroot from its name. A chroot
// absent from the catalog is Deleted; a release past its EOL date is Preserved;
// everything else is Active. Rolling targets (rawhide, branched) are always
// Active.
func Classify(name string, inCatalog bool) State {
	if !inCatalog {
		return Deleted
	}
	info, err := Parse(name)
	if err != nil {
		return Active // unknown shape: do not mislabel
	}
	if evergreen[info.Version] {
		return Active
	}
	date, ok := eol[info.Distro][info.Version]
	if !ok {
		return Active // unknown release: assume current rather than guess EOL
	}
	if afterEOL(date) {
		return Preserved
	}
	return Active
}

// IsActive reports whether a chroot accepts new builds.
func IsActive(name string, inCatalog bool) bool {
	return Classify(name, inCatalog) == Active
}
