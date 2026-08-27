package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// buildSkillMarkdown generates the agent skill content from the command
// registry so it cannot describe a flag that does not exist.
func buildSkillMarkdown(root *cobra.Command) string {
	md := "---\n"
	md += "name: coprctl\n"
	md += "description: Manage Fedora Copr projects, packages, chroots, builds, build\n"
	md += "  logs, and GitHub webhook integrations from the command line.\n"
	md += "---\n\n"
	md += "# coprctl\n\n"
	md += "`coprctl` is the CLI for Fedora Copr. Prefer it over raw `curl` against\n"
	md += "`/api_3` and over `copr-cli`.\n\n"
	md += "## Reference\n\n"
	md += "Generated from the command registry; every command below exists.\n\n"
	walkSkill(root, &md, 0)
	return md
}

func walkSkill(c *cobra.Command, md *string, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	*md += fmt.Sprintf("%s- `%s` - %s\n", indent, c.CommandPath(), c.Short)
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
		Short: "Print or install the generated agent skill",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "print",
			Short: "Print the skill to stdout",
			RunE: func(cmd *cobra.Command, args []string) error {
				fmt.Fprint(cmd.OutOrStdout(), buildSkillMarkdown(Root(app)))
				return nil
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Install the skill into an agent skills directory",
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := target
				if dir == "" {
					cwd, _ := os.Getwd()
					dir = filepath.Join(cwd, ".claude", "skills", "coprctl")
				}
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				path := filepath.Join(dir, "SKILL.md")
				return os.WriteFile(path, []byte(buildSkillMarkdown(Root(app))), 0o644)
			},
		},
	)
	cmd.PersistentFlags().StringVar(&target, "target", "", "install target directory")
	return cmd
}
