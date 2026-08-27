// Package detect implements read-only source project detection: parse the spec
// and git signals, infer a manifest draft, and report what is uncertain. It is
// the primitive that makes init and sync agent-usable without any mutation.
package detect

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/abn/coprctl/internal/manifest"
)

// SpecInfo is the parsed information from a single .spec file.
type SpecInfo struct {
	Path            string `json:"path"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Summary         string `json:"summary"`
	Homepage        string `json:"homepage"`
	License         string `json:"license"`
	BuildArch       string `json:"buildarch,omitempty"`
	NetworkInBuild  bool   `json:"network_in_build"`
	HasAutorelease  bool   `json:"has_autorelease"`
	Method          string `json:"method"`
	Source0IsURL    bool   `json:"source0_is_url"`
	RHELConditional bool   `json:"rhel_conditional"`
	Uncertain       bool   `json:"uncertain,omitempty"`
}

// Result is the complete detection output.
type Result struct {
	RepoDir       string             `json:"repo_dir"`
	Forge         string             `json:"forge,omitempty"`
	RepoName      string             `json:"repo_name,omitempty"`
	CloneURL      string             `json:"clone_url,omitempty"`
	DefaultBranch string             `json:"default_branch,omitempty"`
	Specs         []SpecInfo         `json:"specs"`
	HasTito       bool               `json:"has_tito"`
	HasCoprMake   bool               `json:"has_copr_make"`
	Proposed      *manifest.Manifest `json:"proposed,omitempty"`
	Decisions     []Decision         `json:"decisions_required"`
	Warnings      []string           `json:"warnings"`
}

// Decision is a field the tool refuses to guess, with the flag that answers it.
type Decision struct {
	Field    string   `json:"field"`
	Reason   string   `json:"reason"`
	Proposal []string `json:"proposal,omitempty"`
	Flag     string   `json:"flag"`
}

var (
	reName        = regexp.MustCompile(`(?m)^Name:\s*(.+)$`)
	reVersion     = regexp.MustCompile(`(?m)^Version:\s*(.+)$`)
	reSummary     = regexp.MustCompile(`(?m)^Summary:\s*(.+)$`)
	reHomepage    = regexp.MustCompile(`(?m)^URL:\s*(.+)$`)
	reLicense     = regexp.MustCompile(`(?m)^License:\s*(.+)$`)
	reBuildArch   = regexp.MustCompile(`(?m)^BuildArch:\s*(\S+)`)
	reSource0     = regexp.MustCompile(`(?m)^Source0?:\s*(\S+)`)
	reNetwork     = regexp.MustCompile(`(?mi)(go mod download|cargo fetch|npm (ci|install)|pip install|wget|curl)`)
	reRHEL        = regexp.MustCompile(`(?m)%\{?\??rhel`)
	reAutorelease = regexp.MustCompile(`%autorelease|%autochangelog`)
)

// specDirs are the conventional locations searched for spec files.
var specDirs = []string{".", "rpm", "dist", "packaging", "contrib", ".rpm"}

// Detect scans the repository rooted at path and returns the inferred picture.
// readGit controls whether git signals are read (disable in tests).
func Detect(path string, readGit bool) (*Result, error) {
	res := &Result{RepoDir: path}
	abs, err := filepath.Abs(path)
	if err == nil {
		path = abs
	}
	res.RepoDir = path

	// Find spec files.
	seen := map[string]bool{}
	for _, d := range specDirs {
		entries, err := os.ReadDir(filepath.Join(path, d))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".spec") {
				fp := filepath.Join(d, e.Name())
				if !seen[fp] {
					seen[fp] = true
					si, err := parseSpec(filepath.Join(path, fp))
					if err == nil {
						si.Path = fp
						res.Specs = append(res.Specs, si)
					}
				}
			}
		}
	}
	sort.Slice(res.Specs, func(i, j int) bool { return res.Specs[i].Path < res.Specs[j].Path })

	// Git signals.
	if readGit {
		res.readGit(path)
	}

	// Method and tito detection.
	if _, err := os.Stat(filepath.Join(path, ".tito")); err == nil {
		res.HasTito = true
	}
	if _, err := os.Stat(filepath.Join(path, ".copr", "Makefile")); err == nil {
		res.HasCoprMake = true
	}
	for i := range res.Specs {
		if res.HasTito {
			res.Specs[i].Method = "tito"
		} else if res.HasCoprMake {
			res.Specs[i].Method = "make_srpm"
		} else {
			res.Specs[i].Method = "rpkg"
		}
	}

	res.Proposed = res.buildProposal()
	return res, nil
}

func (r *Result) readGit(path string) {
	// Best-effort: read the origin remote from .git/config.
	cfg := filepath.Join(path, ".git", "config")
	data, err := os.ReadFile(cfg)
	if err != nil {
		return
	}
	// Find [remote "origin"] section and its url.
	lines := strings.Split(string(data), "\n")
	inOrigin := false
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "[") {
			inOrigin = strings.Contains(ln, "remote \"origin\"")
			continue
		}
		if inOrigin && strings.HasPrefix(ln, "url") {
			if eq := strings.Index(ln, "="); eq > 0 {
				r.CloneURL = strings.Trim(strings.TrimSpace(ln[eq+1:]), `"'`)
				break
			}
		}
	}
	if r.CloneURL == "" {
		return
	}
	r.Forge = detectForge(r.CloneURL)
	r.RepoName = repoName(r.CloneURL)
	r.DefaultBranch = "main"
}

func detectForge(url string) string {
	switch {
	case strings.Contains(url, "github.com"):
		return "github"
	case strings.Contains(url, "gitlab.com"):
		return "gitlab"
	case strings.Contains(url, "pagure.io"):
		return "pagure"
	}
	return "other"
}

func repoName(url string) string {
	u := strings.TrimSuffix(url, ".git")
	u = strings.TrimSuffix(u, "/")
	i := strings.LastIndex(u, "/")
	if i < 0 {
		return u
	}
	return u[i+1:]
}

func parseSpec(path string) (SpecInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return SpecInfo{}, err
	}
	defer f.Close()
	var body strings.Builder
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		body.WriteString(sc.Text())
		body.WriteString("\n")
	}
	if err := sc.Err(); err != nil {
		return SpecInfo{}, err
	}
	text := body.String()
	si := SpecInfo{
		Name:            firstMatch(reName, text),
		Version:         firstMatch(reVersion, text),
		Summary:         firstMatch(reSummary, text),
		Homepage:        firstMatch(reHomepage, text),
		License:         firstMatch(reLicense, text),
		BuildArch:       firstMatch(reBuildArch, text),
		NetworkInBuild:  reNetwork.MatchString(text),
		HasAutorelease:  reAutorelease.MatchString(text),
		RHELConditional: reRHEL.MatchString(text),
	}
	if src := firstMatch(reSource0, text); src != "" {
		si.Source0IsURL = strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://")
	}
	return si, nil
}

func firstMatch(re *regexp.Regexp, s string) string {
	if m := re.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

// buildProposal constructs a manifest draft, marking uncertain fields and
// recording which paths were inferred.
func (r *Result) buildProposal() *manifest.Manifest {
	m := &manifest.Manifest{
		APIVersion: "coprctl/v1",
		Kind:       "Project",
		Metadata: manifest.Metadata{
			Name:    r.RepoName,
			Profile: "",
		},
	}
	if len(r.Specs) > 0 {
		s := r.Specs[0]
		m.Spec.Description = s.Summary
		m.Spec.Homepage = s.Homepage
		// Copr's SCM source resolves `spec` as a basename inside `subdirectory`,
		// so split the detected path accordingly.
		dir, base := filepath.Split(s.Path)
		pkg := manifest.Package{
			Name: s.Name,
			Source: manifest.Source{
				Type:       "scm",
				CloneURL:   r.CloneURL,
				Committish: r.DefaultBranch,
				Spec:       base,
				Method:     s.Method,
			},
			AutoRebuild: r.Forge != "" && r.Forge != "other",
		}
		if dir != "" && dir != "./" {
			pkg.Source.Subdirectory = strings.TrimSuffix(dir, "/")
		}
		m.Spec.Packages = append(m.Spec.Packages, pkg)
		// Record the inferred paths so sync only auto-updates these.
		m.XCoprctl.Inferred = []string{
			"spec.description",
			"spec.homepage",
			"packages[" + s.Name + "].source.cloneUrl",
			"packages[" + s.Name + "].source.committish",
			"packages[" + s.Name + "].source.spec",
			"packages[" + s.Name + "].source.subdirectory",
		}
	}
	// Chroots are never auto-selected; always require a decision.
	r.Decisions = append(r.Decisions, Decision{
		Field:  "spec.chroots.enabled",
		Reason: "intent not derivable from spec conditionals",
		Flag:   "--chroot",
	})
	return m
}
