// Package ref implements the single reference parser used by every command.
// One grammar: owner/project[:dir][/segment], plus bare names, group owners,
// and build references. Every command resolves its positional argument here,
// so a parsing bug is a bug in every command.
package ref

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Kind classifies what a reference points at.
type Kind int

const (
	// KindProject is an owner/project (with optional :dir).
	KindProject Kind = iota
	// KindPackage is owner/project/package.
	KindPackage
	// KindProjectChroot is owner/project/chroot.
	KindProjectChroot
	// KindBuild is a bare build id or build/<id>.
	KindBuild
	// KindBuildChroot is <buildid>/<chroot>.
	KindBuildChroot
)

// Ref is a parsed reference.
type Ref struct {
	Kind Kind

	Owner   string // owner (group owners keep the leading @)
	Project string
	Dir     string // side repo / project directory, from :dir
	Segment string // package name or chroot, depending on Kind

	BuildID  int
	BuildCht string // chroot within a build, when Kind == KindBuildChroot

	// Source is the original input string.
	Source string
}

// String returns the canonical owner/project[:dir] form (no segment).
func (r Ref) String() string {
	s := r.Owner + "/" + r.Project
	if r.Dir != "" {
		s += ":" + r.Dir
	}
	return s
}

// ProjectRef returns the owner/project portion regardless of kind.
func (r Ref) ProjectRef() string {
	return r.Owner + "/" + r.Project
}

var (
	// segment is a project, package, or chroot name.
	seg = `[A-Za-z0-9_.\-]+`
	// dir is a project directory / side repo name, which may nest via colons
	// (e.g. pr:123, custom:suffix).
	dirSeg = `[A-Za-z0-9_.:\-]+`
	// owner may be a group (@name).
	owner = `@?` + seg
	// build: <int> or build/<int>.
	buildRe = regexp.MustCompile(`^(?:build/)?([0-9]+)$`)
	// build chroot: <int>/<chroot>.
	buildChrootRe = regexp.MustCompile(`^([0-9]+)/(` + seg + `)$`)
	// project with optional dir: owner/project[:dir].
	projectRe = regexp.MustCompile(`^(` + owner + `)/(` + seg + `)(?::(` + dirSeg + `))?$`)
	// chroot grammar: <distro>-<version>-<arch>.
	chrootRe = regexp.MustCompile(`^[A-Za-z0-9]+-[0-9]+-[A-Za-z0-9_]+$`)
	// bare name (single token, no slash, not a build id).
	bareRe = regexp.MustCompile(`^` + seg + `$`)
)

// chrootCatalog is an injectable predicate used to disambiguate a
// three-segment reference between a package and a project chroot. It is set by
// callers that have access to the cached chroot catalog; when nil, the fallback
// grammar match is used.
var chrootCatalog func(name string) bool

// SetChrootCatalog installs a predicate that reports whether a name is a known
// mock chroot in the instance catalog.
func SetChrootCatalog(pred func(name string) bool) { chrootCatalog = pred }

// IsChrootName reports whether name looks like a chroot.
func IsChrootName(name string) bool {
	if chrootCatalog != nil && chrootCatalog(name) {
		return true
	}
	return chrootRe.MatchString(name)
}

// Options controls three-segment disambiguation.
type Options struct {
	ForcePackage bool // force the third segment to be a package
	ForceChroot  bool // force the third segment to be a chroot
}

// Parse resolves a reference string. When opts is nil, defaults apply.
func Parse(input string, opts *Options) (Ref, error) {
	if opts == nil {
		opts = &Options{}
	}
	in := strings.TrimSpace(input)
	if in == "" {
		return Ref{}, fmt.Errorf("empty reference")
	}

	// Build chroot: <int>/<chroot>.
	if m := buildChrootRe.FindStringSubmatch(in); m != nil {
		id, _ := strconv.Atoi(m[1])
		return Ref{Kind: KindBuildChroot, BuildID: id, BuildCht: m[2], Source: input}, nil
	}
	// Build: <int> or build/<int>.
	if m := buildRe.FindStringSubmatch(in); m != nil {
		id, _ := strconv.Atoi(m[1])
		return Ref{Kind: KindBuild, BuildID: id, Source: input}, nil
	}
	// Bare name: a single project name owned by the authenticated user. The
	// owner is left empty and resolved by the caller.
	if bareRe.MatchString(in) {
		return Ref{Kind: KindProject, Project: in, Source: input}, nil
	}
	// Three-segment: owner/project/segment.
	if i := strings.Index(in, "/"); i > 0 {
		rest := in[i+1:]
		if j := strings.Index(rest, "/"); j > 0 {
			r := Ref{
				Kind:    KindPackage,
				Owner:   in[:i],
				Project: rest[:j],
				Segment: rest[j+1:],
				Source:  input,
			}
			switch {
			case opts.ForceChroot:
				r.Kind = KindProjectChroot
			case opts.ForcePackage:
				r.Kind = KindPackage
			case IsChrootName(r.Segment):
				r.Kind = KindProjectChroot
			}
			return r, nil
		}
	}
	// Project: owner/project[:dir].
	if m := projectRe.FindStringSubmatch(in); m != nil {
		return Ref{
			Kind:    KindProject,
			Owner:   m[1],
			Project: m[2],
			Dir:     m[3],
			Source:  input,
		}, nil
	}

	return Ref{}, fmt.Errorf("unrecognized reference %q", input)
}
