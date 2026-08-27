package cli

import (
	"github.com/spf13/cobra"
)

// newCompletionCmd wires Cobra's generated shell completions into the root.
// Completions are generated from the same command registry as everything else.
func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "bash",
			Short: "Generate bash completion",
			RunE: func(c *cobra.Command, args []string) error {
				return c.Root().GenBashCompletionV2(c.OutOrStdout(), true)
			},
		},
		&cobra.Command{
			Use:   "zsh",
			Short: "Generate zsh completion",
			RunE: func(c *cobra.Command, args []string) error {
				return c.Root().GenZshCompletion(c.OutOrStdout())
			},
		},
		&cobra.Command{
			Use:   "fish",
			Short: "Generate fish completion",
			RunE: func(c *cobra.Command, args []string) error {
				return c.Root().GenFishCompletion(c.OutOrStdout(), true)
			},
		},
		&cobra.Command{
			Use:   "powershell",
			Short: "Generate powershell completion",
			RunE: func(c *cobra.Command, args []string) error {
				return c.Root().GenPowerShellCompletionWithDesc(c.OutOrStdout())
			},
		},
	)
	return cmd
}
