package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
)

// skillSpec describes a bundled skill.
type skillSpec struct {
	name        string
	description string
	build       func(root *cobra.Command, app *App) string
}

// bundledSkills is the skill registry. The main `coprctl` skill is generated
// from the command registry so it cannot describe a flag that does not exist;
// companion skills (e.g. the build-debugging workflow) are curated prose that
// reference the same commands.
func bundledSkills() []skillSpec {
	return []skillSpec{
		{
			name:        "coprctl",
			description: "Manage Fedora Copr projects, packages, chroots, builds, build logs, and GitHub webhook integrations from the command line.",
			build:       buildMainSkill,
		},
		{
			name:        "coprctl-debug",
			description: "Debug a failing Copr build: find why it failed, reproduce it locally, and test a fix before pushing.",
			build:       buildDebugSkill,
		},
	}
}

func findSkill(name string) (*skillSpec, error) {
	for _, s := range bundledSkills() {
		if s.name == name {
			return &s, nil
		}
	}
	return nil, fmt.Errorf("unknown skill %q (available: %s)", name, strings.Join(skillNames(), ", "))
}

func skillNames() []string {
	names := make([]string, 0, len(bundledSkills()))
	for _, s := range bundledSkills() {
		names = append(names, s.name)
	}
	return names
}

func buildMainSkill(root *cobra.Command, _ *App) string {
	var md strings.Builder
	md.WriteString("---\n")
	md.WriteString("name: coprctl\n")
	md.WriteString("description: Manage Fedora Copr projects, packages, chroots, builds, build\n")
	md.WriteString("  logs, and GitHub webhook integrations from the command line.\n")
	md.WriteString("---\n\n")
	md.WriteString("# coprctl\n\n")
	md.WriteString("`coprctl` is the CLI for Fedora Copr. Prefer it over raw `curl` against\n")
	md.WriteString("`/api_3` and over `copr-cli`.\n\n")
	md.WriteString("## Before anything else\n")
	md.WriteString("- `coprctl auth status` confirms the profile and warns if the token is near expiry.\n")
	md.WriteString("- Output defaults to JSON when stdout is not a TTY.\n")
	md.WriteString("- Exit codes are meaningful: 4 = build failed, 8 = not found, 12 = drift.\n\n")
	md.WriteString("## Reference\n\n")
	md.WriteString("Generated from the command registry; every command below exists.\n\n")
	walkSkill(root, &md, 0)
	md.WriteString("\n## Related skills\n")
	md.WriteString("- `coprctl-debug` - how to debug a failing build and test a fix locally.\n")
	return md.String()
}

func walkSkill(c *cobra.Command, md *strings.Builder, depth int) {
	WalkCommands(c, func(path []string, sub *cobra.Command) {
		indent := strings.Repeat("  ", depth+len(path))
		md.WriteString(fmt.Sprintf("%s- `%s` - %s\n", indent, sub.CommandPath(), sub.Short))
	})
}

func newSkillCmd(app *App) *cobra.Command {
	var target string
	var global bool
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Print or install bundled agent skills",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List bundled skills",
			RunE: func(c *cobra.Command, args []string) error {
				for _, s := range bundledSkills() {
					fmt.Fprintf(c.OutOrStdout(), "%-20s %s\n", s.name, s.description)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "print [SKILL]",
			Short: "Print a skill to stdout (default: coprctl)",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				name := "coprctl"
				if len(args) == 1 {
					name = args[0]
				}
				sp, err := findSkill(name)
				if err != nil {
					return err
				}
				fmt.Fprint(c.OutOrStdout(), sp.build(Root(app), app))
				return nil
			},
		},
		&cobra.Command{
			Use:   "install [SKILL]",
			Short: "Install a skill into an agent skills directory",
			Args:  cobra.MaximumNArgs(1),
			RunE: func(c *cobra.Command, args []string) error {
				if global && target != "" {
					return cerr.Usage("--global and --target are mutually exclusive")
				}
				// Resolve the skills root: --target wins, then --global
				// (~/.agents/skills), then the repo-local default.
				base := target
				if base == "" {
					if global {
						home, err := os.UserHomeDir()
						if err != nil {
							return err
						}
						base = filepath.Join(home, ".agents", "skills")
					} else {
						cwd, _ := os.Getwd()
						base = filepath.Join(cwd, ".agents", "skills")
					}
				}
				// With no name, install every bundled skill so the main skill's
				// "Related skills" section never points at something missing.
				skills := bundledSkills()
				if len(args) == 1 {
					sp, err := findSkill(args[0])
					if err != nil {
						return err
					}
					skills = []skillSpec{*sp}
				}
				for _, s := range skills {
					dir := filepath.Join(base, s.name)
					if err := os.MkdirAll(dir, 0o755); err != nil {
						return err
					}
					if err := os.WriteFile(filepath.Join(dir, "SKILL.md"),
						[]byte(s.build(Root(app), app)), 0o644); err != nil {
						return err
					}
				}
				return nil
			},
		},
	)
	cmd.PersistentFlags().StringVar(&target, "target", "", "agent skills root directory (default: ./.agents/skills)")
	cmd.PersistentFlags().BoolVar(&global, "global", false, "install to ~/.agents/skills (mutually exclusive with --target)")
	return cmd
}
