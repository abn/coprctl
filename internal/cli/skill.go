package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
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
	indent := strings.Repeat("  ", depth)
	md.WriteString(fmt.Sprintf("%s- `%s` - %s\n", indent, c.CommandPath(), c.Short))
	for _, sub := range c.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		walkSkill(sub, md, depth+1)
	}
}

func newSkillCmd(app *App) *cobra.Command {
	var target string
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
				name := "coprctl"
				if len(args) == 1 {
					name = args[0]
				}
				sp, err := findSkill(name)
				if err != nil {
					return err
				}
				// Install namespaced under the target's skills root.
				base := target
				if base == "" {
					cwd, _ := os.Getwd()
					base = filepath.Join(cwd, ".claude", "skills")
				}
				dir := filepath.Join(base, sp.name)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(dir, "SKILL.md"),
					[]byte(sp.build(Root(app), app)), 0o644)
			},
		},
	)
	cmd.PersistentFlags().StringVar(&target, "target", "", "agent skills root directory")
	return cmd
}
