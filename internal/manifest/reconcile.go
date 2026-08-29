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
	Description  string
	Instructions string
	Homepage     string
	Contact      string
	DevelMode    bool
	EnableNet    bool
	Chroots      []string
	Packages     []copr.Package
	Permissions  Permissions
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
	perms, err := e.Client.ListPermissions(ctx, owner, project)
	if err != nil {
		return nil, err
	}
	return &LiveState{
		Description:  p.Description,
		Instructions: p.Instructions,
		Homepage:     p.Homepage,
		Contact:      p.Contact,
		DevelMode:    p.DevelMode,
		EnableNet:    p.EnableNet,
		Chroots:      sortedKeys(p.ChrootRepos),
		Packages:     pkgs,
		Permissions:  permissionsFromLive(perms),
	}, nil
}

// permissionsFromLive extracts the approved admins and builders from the live
// permission map, as sorted lists.
func permissionsFromLive(live copr.Permissions) Permissions {
	var p Permissions
	for user, set := range live {
		if set.Builder == copr.PermissionApproved {
			p.Builders = append(p.Builders, user)
		}
		if set.Admin == copr.PermissionApproved {
			p.Admins = append(p.Admins, user)
		}
	}
	sort.Strings(p.Builders)
	sort.Strings(p.Admins)
	return p
}

// PermissionSetFromManifest builds the copr permission map for the users the
// manifest lists. Only those users are touched: a user listed as a builder
// gets the builder role approved, an admin the admin role; unlisted users keep
// their live permissions.
func PermissionSetFromManifest(p Permissions) copr.Permissions {
	perms := make(copr.Permissions)
	for _, u := range p.Builders {
		set := perms[u]
		set.Builder = copr.PermissionApproved
		perms[u] = set
	}
	for _, u := range p.Admins {
		set := perms[u]
		set.Admin = copr.PermissionApproved
		perms[u] = set
	}
	return perms
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
			Description:  live.Description,
			Instructions: live.Instructions,
			Homepage:     live.Homepage,
			Contact:      live.Contact,
			Settings:     Settings{DevelMode: live.DevelMode, EnableNet: live.EnableNet},
			Chroots:      Chroots{Enabled: live.Chroots},
			Permissions:  live.Permissions,
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
	if m.Spec.Instructions != live.Instructions {
		diffs = append(diffs, Diff{Path: "spec.instructions", Manifest: m.Spec.Instructions, Live: live.Instructions})
	}
	if m.Spec.Homepage != live.Homepage {
		diffs = append(diffs, Diff{Path: "spec.homepage", Manifest: m.Spec.Homepage, Live: live.Homepage})
	}
	if m.Spec.Settings.DevelMode != live.DevelMode {
		diffs = append(diffs, Diff{Path: "spec.settings.develMode",
			Manifest: fmt.Sprintf("%v", m.Spec.Settings.DevelMode), Live: fmt.Sprintf("%v", live.DevelMode)})
	}
	if m.Spec.Settings.EnableNet != live.EnableNet {
		diffs = append(diffs, Diff{Path: "spec.settings.enableNet",
			Manifest: fmt.Sprintf("%v", m.Spec.Settings.EnableNet), Live: fmt.Sprintf("%v", live.EnableNet)})
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
	// Permission drift only applies when the manifest manages permissions; a
	// manifest that omits them leaves live permissions untouched.
	if len(m.Spec.Permissions.Builders) > 0 || len(m.Spec.Permissions.Admins) > 0 {
		diffPermissionSet("spec.permissions.builders", m.Spec.Permissions.Builders, live.Permissions.Builders, &diffs)
		diffPermissionSet("spec.permissions.admins", m.Spec.Permissions.Admins, live.Permissions.Admins, &diffs)
	}
	return diffs, nil
}

// diffPermissionSet appends diffs for names present in one set but not the
// other.
func diffPermissionSet(path string, want, live []string, diffs *[]Diff) {
	wantSet := make(map[string]bool, len(want))
	for _, u := range want {
		wantSet[u] = true
	}
	liveSet := make(map[string]bool, len(live))
	for _, u := range live {
		liveSet[u] = true
	}
	for _, u := range want {
		if !liveSet[u] {
			*diffs = append(*diffs, Diff{Path: path, Manifest: u, Live: "(absent)"})
		}
	}
	for _, u := range live {
		if !wantSet[u] {
			*diffs = append(*diffs, Diff{Path: path, Manifest: "(absent)", Live: u})
		}
	}
}
