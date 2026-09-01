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

// LiveState is the live snapshot of a project used for diff and export. It
// carries the readable and reconcilable settings only: create-only fields
// (persistent, storage) and add+edit/write-only fields (multilib, fedoraReview,
// deleteAfterDays, runtimeDependencies, package settings) are excluded because
// apply cannot converge them on an existing project.
type LiveState struct {
	Description                string
	Instructions               string
	Homepage                   string
	Contact                    string
	DevelMode                  bool
	EnableNet                  bool
	AutoPrune                  bool
	Bootstrap                  string
	Isolation                  string
	ModuleHotfixes             bool
	Appstream                  bool
	PackitForgeProjectsAllowed []string
	FollowFedoraBranching      bool
	RepoPriority               int
	UnlistedOnHomepage         bool
	Chroots                    []string
	Packages                   []copr.Package
	Permissions                Permissions
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
		Description:                p.Description,
		Instructions:               p.Instructions,
		Homepage:                   p.Homepage,
		Contact:                    p.Contact,
		DevelMode:                  p.DevelMode,
		EnableNet:                  p.EnableNet,
		AutoPrune:                  p.AutoPrune,
		Bootstrap:                  p.Bootstrap,
		Isolation:                  p.Isolation,
		ModuleHotfixes:             p.ModuleHotfixes,
		Appstream:                  p.Appstream,
		PackitForgeProjectsAllowed: p.PackitForgeProjectsAllowed,
		FollowFedoraBranching:      p.FollowFedoraBranching,
		RepoPriority:               p.RepoPriority,
		UnlistedOnHomepage:         p.UnlistedOnHomepage,
		Chroots:                    sortedKeys(p.ChrootRepos),
		Packages:                   pkgs,
		Permissions:                permissionsFromLive(perms),
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
			Settings: Settings{
				DevelMode:                  live.DevelMode,
				EnableNet:                  live.EnableNet,
				AutoPrune:                  live.AutoPrune,
				Bootstrap:                  live.Bootstrap,
				Isolation:                  live.Isolation,
				ModuleHotfixes:             live.ModuleHotfixes,
				Appstream:                  live.Appstream,
				PackitForgeProjectsAllowed: live.PackitForgeProjectsAllowed,
				FollowFedoraBranching:      live.FollowFedoraBranching,
				RepoPriority:               live.RepoPriority,
				UnlistedOnHomepage:         live.UnlistedOnHomepage,
			},
			Chroots:     Chroots{Enabled: live.Chroots},
			Permissions: live.Permissions,
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
		pkg := Package{Name: p.Name, Source: src}
		if p.AutoRebuild {
			ar := true
			pkg.AutoRebuild = &ar
		}
		m.Spec.Packages = append(m.Spec.Packages, pkg)
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
	// The readable settings are compared only when the manifest declares them
	// (non-zero). Manifest zero-values (autoPrune false, empty
	// bootstrap/isolation, followFedoraBranching false) diverge from the live
	// defaults (auto_prune true, bootstrap "default", isolation "default",
	// follow_fedora_branching true), and apply is declared-only, so an
	// unconditional comparison would make apply-then-diff permanently red on
	// minimal manifests.
	s := m.Spec.Settings
	for _, d := range []struct {
		path string
		want bool
		live bool
	}{
		{"spec.settings.autoPrune", s.AutoPrune, live.AutoPrune},
		{"spec.settings.moduleHotfixes", s.ModuleHotfixes, live.ModuleHotfixes},
		{"spec.settings.appstream", s.Appstream, live.Appstream},
		{"spec.settings.followFedoraBranching", s.FollowFedoraBranching, live.FollowFedoraBranching},
		{"spec.settings.unlistedOnHomepage", s.UnlistedOnHomepage, live.UnlistedOnHomepage},
	} {
		if d.want && d.want != d.live {
			diffs = append(diffs, Diff{Path: d.path,
				Manifest: fmt.Sprintf("%v", d.want), Live: fmt.Sprintf("%v", d.live)})
		}
	}
	for _, d := range []struct {
		path string
		want string
		live string
	}{
		{"spec.settings.bootstrap", s.Bootstrap, live.Bootstrap},
		{"spec.settings.isolation", s.Isolation, live.Isolation},
	} {
		if d.want != "" && d.want != d.live {
			diffs = append(diffs, Diff{Path: d.path, Manifest: d.want, Live: d.live})
		}
	}
	if s.RepoPriority != 0 && s.RepoPriority != live.RepoPriority {
		diffs = append(diffs, Diff{Path: "spec.settings.repoPriority",
			Manifest: fmt.Sprintf("%d", s.RepoPriority), Live: fmt.Sprintf("%d", live.RepoPriority)})
	}
	if len(s.PackitForgeProjectsAllowed) > 0 {
		diffStrings("spec.settings.packitForgeProjectsAllowed", s.PackitForgeProjectsAllowed, live.PackitForgeProjectsAllowed, &diffs)
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
		diffStrings("spec.permissions.builders", m.Spec.Permissions.Builders, live.Permissions.Builders, &diffs)
		diffStrings("spec.permissions.admins", m.Spec.Permissions.Admins, live.Permissions.Admins, &diffs)
	}
	return diffs, nil
}

// diffStrings appends diffs for entries present in one set but not the other.
func diffStrings(path string, want, live []string, diffs *[]Diff) {
	wantSet := make(map[string]bool, len(want))
	for _, s := range want {
		wantSet[s] = true
	}
	liveSet := make(map[string]bool, len(live))
	for _, s := range live {
		liveSet[s] = true
	}
	for _, s := range want {
		if !liveSet[s] {
			*diffs = append(*diffs, Diff{Path: path, Manifest: s, Live: "(absent)"})
		}
	}
	for _, s := range live {
		if !wantSet[s] {
			*diffs = append(*diffs, Diff{Path: path, Manifest: "(absent)", Live: s})
		}
	}
}
