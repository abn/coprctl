// Package manifest implements declarative project state (copr.yaml): schema,
// validate, diff, apply, and export. An agent writes a file and reconciles
// instead of composing imperative calls.
package manifest

import (
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

// chrootDenylistPattern matches upstream REGEX_CHROOT_DENYLIST (forms.py). The
// check is deliberately weaker than upstream, which also rejects patterns that
// match no active chroot or all chroots; offline tolerance requires local-only
// checks, and the server re-validates against active chroots on apply.
var chrootDenylistPattern = regexp.MustCompile(`^[a-z0-9-_*.+]+$`)

// Manifest is the root document of copr.yaml.
type Manifest struct {
	APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
	Kind       string   `yaml:"kind" json:"kind"`
	Metadata   Metadata `yaml:"metadata" json:"metadata"`
	Spec       Spec     `yaml:"spec" json:"spec"`
	XCoprctl   XCoprctl `yaml:"x-coprctl,omitempty" json:"x-coprctl,omitempty"`
}

// XCoprctl is the provenance extension: it records which fields were inferred
// from the source repo so sync may only auto-update those paths. Anything not
// listed here is human-owned and never auto-overwritten.
type XCoprctl struct {
	GeneratedBy          string   `yaml:"generatedBy,omitempty" json:"generatedBy,omitempty"`
	DetectionFingerprint string   `yaml:"detectionFingerprint,omitempty" json:"detectionFingerprint,omitempty"`
	Inferred             []string `yaml:"inferred,omitempty" json:"inferred,omitempty"`
}

// Metadata identifies the target project.
type Metadata struct {
	Owner   string `yaml:"owner" json:"owner"`
	Name    string `yaml:"name" json:"name"`
	Profile string `yaml:"profile,omitempty" json:"profile,omitempty"`
}

// Spec is the desired project state.
type Spec struct {
	Description  string      `yaml:"description,omitempty" json:"description,omitempty"`
	Instructions string      `yaml:"instructions,omitempty" json:"instructions,omitempty"`
	Homepage     string      `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Contact      string      `yaml:"contact,omitempty" json:"contact,omitempty"`
	Settings     Settings    `yaml:"settings,omitempty" json:"settings,omitempty"`
	Chroots      Chroots     `yaml:"chroots,omitempty" json:"chroots,omitempty"`
	Packages     []Package   `yaml:"packages,omitempty" json:"packages,omitempty"`
	Permissions  Permissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
}

// Settings are project-level flags.
type Settings struct {
	EnableNet                  bool     `yaml:"enableNet,omitempty" json:"enableNet,omitempty"`
	Appstream                  bool     `yaml:"appstream,omitempty" json:"appstream,omitempty"`
	DevelMode                  bool     `yaml:"develMode,omitempty" json:"develMode,omitempty"`
	AutoPrune                  bool     `yaml:"autoPrune,omitempty" json:"autoPrune,omitempty"`
	UnlistedOnHomepage         bool     `yaml:"unlistedOnHomepage,omitempty" json:"unlistedOnHomepage,omitempty"`
	FollowFedoraBranching      bool     `yaml:"followFedoraBranching,omitempty" json:"followFedoraBranching,omitempty"`
	ModuleHotfixes             bool     `yaml:"moduleHotfixes,omitempty" json:"moduleHotfixes,omitempty"`
	Multilib                   bool     `yaml:"multilib,omitempty" json:"multilib,omitempty"`
	FedoraReview               bool     `yaml:"fedoraReview,omitempty" json:"fedoraReview,omitempty"`
	Isolation                  string   `yaml:"isolation,omitempty" json:"isolation,omitempty"`
	Bootstrap                  string   `yaml:"bootstrap,omitempty" json:"bootstrap,omitempty"`
	RepoPriority               int      `yaml:"repoPriority,omitempty" json:"repoPriority,omitempty"`
	DeleteAfterDays            *int     `yaml:"deleteAfterDays,omitempty" json:"deleteAfterDays,omitempty"`
	AdditionalRepos            []string `yaml:"additionalRepos,omitempty" json:"additionalRepos,omitempty"`
	Persistent                 bool     `yaml:"persistent,omitempty" json:"persistent,omitempty"`
	Storage                    string   `yaml:"storage,omitempty" json:"storage,omitempty"`
	PackitForgeProjectsAllowed []string `yaml:"packitForgeProjectsAllowed,omitempty" json:"packitForgeProjectsAllowed,omitempty"`
	RuntimeDependencies        []string `yaml:"runtimeDependencies,omitempty" json:"runtimeDependencies,omitempty"`
}

// Chroots describes the enabled chroots and per-chroot config.
type Chroots struct {
	Enabled []string                `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Config  map[string]ChrootConfig `yaml:"config,omitempty" json:"config,omitempty"`
}

// ChrootConfig is per-chroot buildroot configuration.
type ChrootConfig struct {
	AdditionalPackages []string `yaml:"additionalPackages,omitempty" json:"additionalPackages,omitempty"`
	AdditionalRepos    []string `yaml:"additionalRepos,omitempty" json:"additionalRepos,omitempty"`
	Modules            []string `yaml:"modules,omitempty" json:"modules,omitempty"`
	With               []string `yaml:"with,omitempty" json:"with,omitempty"`
	Without            []string `yaml:"without,omitempty" json:"without,omitempty"`
	Isolation          string   `yaml:"isolation,omitempty" json:"isolation,omitempty"`
}

// Package is a package source definition.
type Package struct {
	Name   string `yaml:"name" json:"name"`
	Source Source `yaml:"source" json:"source"`
	// AutoRebuild is a pointer so apply can tell a declared value from an
	// absent one: an undeclared autoRebuild must not clobber a live webhook
	// trigger on re-apply, while a declared value round-trips.
	AutoRebuild *bool `yaml:"autoRebuild,omitempty" json:"autoRebuild,omitempty"`
	// MaxBuilds and Timeout are write-only through the API (GET does not echo
	// them); pointers keep a declared zero expressible. ChrootDenylist is a
	// comma-joined list on the wire; an empty slice cannot clear a live
	// denylist, and the pointer form is not worth the schema complexity for a
	// field the API never echoes.
	MaxBuilds      *int     `yaml:"maxBuilds,omitempty" json:"maxBuilds,omitempty"`
	Timeout        *int     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ChrootDenylist []string `yaml:"chrootDenylist,omitempty" json:"chrootDenylist,omitempty"`
}

// Source is a source definition.
type Source struct {
	Type         string `yaml:"type" json:"type"`
	CloneURL     string `yaml:"cloneUrl,omitempty" json:"cloneUrl,omitempty"`
	Committish   string `yaml:"committish,omitempty" json:"committish,omitempty"`
	Subdirectory string `yaml:"subdirectory,omitempty" json:"subdirectory,omitempty"`
	Spec         string `yaml:"spec,omitempty" json:"spec,omitempty"`
	Method       string `yaml:"method,omitempty" json:"method,omitempty"`
}

// Permissions lists builders and admins.
type Permissions struct {
	Builders []string `yaml:"builders,omitempty" json:"builders,omitempty"`
	Admins   []string `yaml:"admins,omitempty" json:"admins,omitempty"`
}

// Parse parses manifest bytes (YAML or JSON).
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	if m.APIVersion == "" || m.Kind == "" {
		return nil, fmt.Errorf("manifest missing apiVersion/kind")
	}
	if m.Metadata.Owner == "" || m.Metadata.Name == "" {
		return nil, fmt.Errorf("manifest missing metadata.owner/name")
	}
	return &m, nil
}

// ValidationIssue is a single validation finding.
type ValidationIssue struct {
	Path   string `json:"path"`
	Level  string `json:"level"` // error | warning
	Detail string `json:"detail"`
}

// Validate performs schema and semantic checks that need no network.
func (m *Manifest) Validate() []ValidationIssue {
	var issues []ValidationIssue
	if m.APIVersion != "coprctl/v1" {
		issues = append(issues, ValidationIssue{Path: "apiVersion", Level: "error",
			Detail: fmt.Sprintf("unsupported apiVersion %q", m.APIVersion)})
	}
	if m.Kind != "Project" {
		issues = append(issues, ValidationIssue{Path: "kind", Level: "error",
			Detail: fmt.Sprintf("unsupported kind %q", m.Kind)})
	}
	// persistent and storage are create-only: the edit API has no field for
	// them, so re-apply cannot converge them on an existing project.
	// additionalRepos is editable upstream but the client does not model it, so
	// it is likewise never sent on apply. Flag all three as warnings rather
	// than errors: the manifest is well-formed, but the declared state is not
	// reconciled after creation.
	s := &m.Spec.Settings
	for _, u := range []struct {
		path    string
		present bool
		detail  string
	}{
		{"spec.settings.additionalRepos", len(s.AdditionalRepos) > 0,
			"not modeled by the Copr client; ignored on apply"},
		{"spec.settings.persistent", s.Persistent,
			"only applied on project creation; the edit API has no field"},
		{"spec.settings.storage", s.Storage != "",
			"only applied on project creation; the edit API has no field"},
	} {
		if u.present {
			issues = append(issues, ValidationIssue{Path: u.path, Level: "warning", Detail: u.detail})
		}
	}
	// Upstream forbids combining the two: a persistent project cannot be set
	// to auto-delete (forms.py CoprForm).
	if s.Persistent && s.DeleteAfterDays != nil {
		issues = append(issues, ValidationIssue{Path: "spec.settings", Level: "error",
			Detail: "persistent cannot be combined with deleteAfterDays"})
	}
	// Upstream bound: delete_after_days -1..720 (forms.py CoprForm
	// NumberRange); a value outside it would be rejected by the server.
	if s.DeleteAfterDays != nil && (*s.DeleteAfterDays < -1 || *s.DeleteAfterDays > 720) {
		issues = append(issues, ValidationIssue{Path: "spec.settings.deleteAfterDays", Level: "error",
			Detail: "deleteAfterDays must be in -1..720"})
	}
	for i, p := range m.Spec.Packages {
		path := fmt.Sprintf("spec.packages[%d]", i)
		if p.Name == "" {
			issues = append(issues, ValidationIssue{Path: path + ".name", Level: "error", Detail: "package name required"})
		}
		if p.Source.Type == "" {
			issues = append(issues, ValidationIssue{Path: path + ".source.type", Level: "error", Detail: "source type required"})
		}
		switch p.Source.Type {
		case "scm":
			if p.Source.CloneURL == "" {
				issues = append(issues, ValidationIssue{Path: path + ".source.cloneUrl", Level: "error", Detail: "scm source requires cloneUrl"})
			}
		case "distgit":
			if p.Source.Committish == "" {
				issues = append(issues, ValidationIssue{Path: path + ".source.committish", Level: "error", Detail: "distgit source requires committish"})
			}
		}
		// Upstream bounds: max_builds 0..100 and timeout 0..108000 (forms.py
		// BasePackageForm, config.py MAX_BUILD_TIMEOUT); zero or absent means
		// the default.
		if p.MaxBuilds != nil && (*p.MaxBuilds < 0 || *p.MaxBuilds > 100) {
			issues = append(issues, ValidationIssue{Path: path + ".maxBuilds", Level: "error",
				Detail: "maxBuilds must be in 0..100"})
		}
		if p.Timeout != nil && (*p.Timeout < 0 || *p.Timeout > 108000) {
			issues = append(issues, ValidationIssue{Path: path + ".timeout", Level: "error",
				Detail: "timeout must be in 0..108000"})
		}
		for _, pattern := range p.ChrootDenylist {
			if !chrootDenylistPattern.MatchString(pattern) {
				issues = append(issues, ValidationIssue{Path: path + ".chrootDenylist", Level: "error",
					Detail: fmt.Sprintf("invalid chroot pattern %q", pattern)})
			}
		}
	}
	return issues
}
