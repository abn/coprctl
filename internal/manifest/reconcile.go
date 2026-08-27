package manifest

import (
	"context"
	"fmt"
	"sort"

	"github.com/abn/coprctl/internal/copr"
	"gopkg.in/yaml.v3"
)

// Exporter reads live Copr state.
type Exporter struct {
	Client *copr.Client
}

// LiveState is the live snapshot of a project used for diff and export.
type LiveState struct {
	Description string
	Homepage    string
	Contact     string
	DevelMode   bool
	Chroots     []string
	Packages    []copr.Package
}

// FetchLiveState pulls the current state of a project.
func (e *Exporter) FetchLiveState(ctx context.Context, owner, project string) (*LiveState, error) {
	p, err := e.Client.GetProject(ctx, owner, project)
	if err != nil {
		return nil, err
	}
	pkgs, err := e.Client.ListPackages(ctx, owner, project)
	if err != nil {
		return nil, err
	}
	return &LiveState{
		Description: p.Description,
		Homepage:    p.Homepage,
		Contact:     p.Contact,
		DevelMode:   p.DevelMode,
		Chroots:     sortedKeys(p.ChrootRepos),
		Packages:    pkgs,
	}, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ExportFromLive builds a manifest from live project state.
func ExportFromLive(ctx context.Context, c *copr.Client, owner, project string) (*Manifest, error) {
	live, err := (&Exporter{Client: c}).FetchLiveState(ctx, owner, project)
	if err != nil {
		return nil, err
	}
	m := &Manifest{
		APIVersion: "coprctl/v1",
		Kind:       "Project",
		Metadata:   Metadata{Owner: owner, Name: project},
		Spec: Spec{
			Description: live.Description,
			Homepage:    live.Homepage,
			Contact:     live.Contact,
			Settings:    Settings{DevelMode: live.DevelMode},
			Chroots:     Chroots{Enabled: live.Chroots},
		},
	}
	for _, p := range live.Packages {
		src := Source{Type: string(p.SourceType)}
		switch copr.SourceType(p.SourceType) {
		case copr.SourceSCM:
			src.CloneURL = p.SourceDict["clone_url"]
			src.Committish = p.SourceDict["committish"]
			src.Subdirectory = p.SourceDict["subdirectory"]
			src.Spec = p.SourceDict["spec"]
		case copr.SourceDistGit:
			src.Committish = p.SourceDict["committish"]
		}
		m.Spec.Packages = append(m.Spec.Packages, Package{
			Name:        p.Name,
			AutoRebuild: p.AutoRebuild,
			Source:      src,
		})
	}
	return m, nil
}

// MarshalYAML renders the manifest as YAML.
func (m *Manifest) MarshalYAML() ([]byte, error) {
	return yaml.Marshal(m)
}

// Diff represents a field-level difference between manifest and live state.
type Diff struct {
	Path     string `json:"path"`
	Manifest string `json:"manifest,omitempty"`
	Live     string `json:"live,omitempty"`
}

// DiffAgainst compares the manifest against live state and returns the drift.
// An empty result means in sync.
func (m *Manifest) DiffAgainst(ctx context.Context, c *copr.Client) ([]Diff, error) {
	live, err := (&Exporter{Client: c}).FetchLiveState(ctx, m.Metadata.Owner, m.Metadata.Name)
	if err != nil {
		return nil, err
	}
	var diffs []Diff
	if m.Spec.Description != live.Description {
		diffs = append(diffs, Diff{Path: "spec.description", Manifest: m.Spec.Description, Live: live.Description})
	}
	if m.Spec.Homepage != live.Homepage {
		diffs = append(diffs, Diff{Path: "spec.homepage", Manifest: m.Spec.Homepage, Live: live.Homepage})
	}
	if m.Spec.Settings.DevelMode != live.DevelMode {
		diffs = append(diffs, Diff{Path: "spec.settings.develMode",
			Manifest: fmt.Sprintf("%v", m.Spec.Settings.DevelMode), Live: fmt.Sprintf("%v", live.DevelMode)})
	}
	// Chroot set drift.
	manifestChroots := map[string]bool{}
	for _, ch := range m.Spec.Chroots.Enabled {
		manifestChroots[ch] = true
	}
	liveChroots := map[string]bool{}
	for _, ch := range live.Chroots {
		liveChroots[ch] = true
	}
	for ch := range manifestChroots {
		if !liveChroots[ch] {
			diffs = append(diffs, Diff{Path: "spec.chroots.enabled", Manifest: ch, Live: "(absent)"})
		}
	}
	for ch := range liveChroots {
		if !manifestChroots[ch] {
			diffs = append(diffs, Diff{Path: "spec.chroots.enabled", Manifest: "(absent)", Live: ch})
		}
	}
	return diffs, nil
}
