// Package manifest implements declarative project state (copr.yaml): schema,
// validate, diff, apply, and export. An agent writes a file and reconciles
// instead of composing imperative calls.
package manifest

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

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
	EnableNet             bool     `yaml:"enableNet,omitempty" json:"enableNet,omitempty"`
	Appstream             bool     `yaml:"appstream,omitempty" json:"appstream,omitempty"`
	DevelMode             bool     `yaml:"develMode,omitempty" json:"develMode,omitempty"`
	AutoPrune             bool     `yaml:"autoPrune,omitempty" json:"autoPrune,omitempty"`
	UnlistedOnHomepage    bool     `yaml:"unlistedOnHomepage,omitempty" json:"unlistedOnHomepage,omitempty"`
	FollowFedoraBranching bool     `yaml:"followFedoraBranching,omitempty" json:"followFedoraBranching,omitempty"`
	ModuleHotfixes        bool     `yaml:"moduleHotfixes,omitempty" json:"moduleHotfixes,omitempty"`
	Multilib              bool     `yaml:"multilib,omitempty" json:"multilib,omitempty"`
	FedoraReview          bool     `yaml:"fedoraReview,omitempty" json:"fedoraReview,omitempty"`
	Isolation             string   `yaml:"isolation,omitempty" json:"isolation,omitempty"`
	Bootstrap             string   `yaml:"bootstrap,omitempty" json:"bootstrap,omitempty"`
	RepoPriority          int      `yaml:"repoPriority,omitempty" json:"repoPriority,omitempty"`
	DeleteAfterDays       *int     `yaml:"deleteAfterDays,omitempty" json:"deleteAfterDays,omitempty"`
	AdditionalRepos       []string `yaml:"additionalRepos,omitempty" json:"additionalRepos,omitempty"`
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
	Name        string `yaml:"name" json:"name"`
	Source      Source `yaml:"source" json:"source"`
	AutoRebuild bool   `yaml:"autoRebuild,omitempty" json:"autoRebuild,omitempty"`
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
	// The Copr client does not model these settings, so a manifest that sets
	// them would silently drift. Flag them as warnings rather than errors: the
	// manifest is well-formed, but the declared state is not fully applied.
	s := &m.Spec.Settings
	for _, u := range []struct {
		path    string
		present bool
	}{
		{"spec.settings.appstream", s.Appstream},
		{"spec.settings.autoPrune", s.AutoPrune},
		{"spec.settings.followFedoraBranching", s.FollowFedoraBranching},
		{"spec.settings.moduleHotfixes", s.ModuleHotfixes},
		{"spec.settings.multilib", s.Multilib},
		{"spec.settings.fedoraReview", s.FedoraReview},
		{"spec.settings.isolation", s.Isolation != ""},
		{"spec.settings.bootstrap", s.Bootstrap != ""},
		{"spec.settings.repoPriority", s.RepoPriority != 0},
		{"spec.settings.deleteAfterDays", s.DeleteAfterDays != nil},
		{"spec.settings.additionalRepos", len(s.AdditionalRepos) > 0},
	} {
		if u.present {
			issues = append(issues, ValidationIssue{Path: u.path, Level: "warning",
				Detail: "not supported by the Copr client; ignored on apply"})
		}
	}
	if s.UnlistedOnHomepage {
		issues = append(issues, ValidationIssue{Path: "spec.settings.unlistedOnHomepage", Level: "warning",
			Detail: "only applied on project creation; the edit API has no field"})
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
	}
	return issues
}
