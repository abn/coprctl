package cli

import "github.com/spf13/cobra"

// WalkCommands visits every available command under root in command order,
// root included. The path passed to fn is the sequence of command names from
// root's children down to c; root itself is visited with an empty path.
// Hidden and deprecated commands are skipped, as is any subtree beneath them.
func WalkCommands(root *cobra.Command, fn func(path []string, c *cobra.Command)) {
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		fn(path, c)
		for _, sub := range c.Commands() {
			if !sub.IsAvailableCommand() {
				continue
			}
			walk(sub, append(path, sub.Name()))
		}
	}
	walk(root, nil)
}

// WalkLeaves visits every available leaf command under root: commands with no
// subcommands at all. The path has the same shape as WalkCommands.
func WalkLeaves(root *cobra.Command, fn func(path []string, c *cobra.Command)) {
	WalkCommands(root, func(path []string, c *cobra.Command) {
		if len(c.Commands()) == 0 {
			fn(path, c)
		}
	})
}
