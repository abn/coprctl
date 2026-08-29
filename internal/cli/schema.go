package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/render"
)

func newSchemaCmd(app *App) *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Emit the command tree as JSON or markdown",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch format {
			case "", "json":
				s := buildSchema(Root(app))
				return render.Render(cmd.OutOrStdout(), render.FormatJSON, s)
			case "mcp":
				tools := schemaMCP(Root(app))
				return render.Render(cmd.OutOrStdout(), render.FormatJSON, tools)
			case "markdown":
				fmt.Fprintln(cmd.OutOrStdout(), "---")
				fmt.Fprintln(cmd.OutOrStdout(), "type: Reference")
				fmt.Fprintln(cmd.OutOrStdout(), "title: Command reference")
				fmt.Fprintln(cmd.OutOrStdout(), "description: The full coprctl command tree, generated from the registry.")
				fmt.Fprintln(cmd.OutOrStdout(), "---")
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), "# coprctl commands")
				walkSchema(Root(app), cmd, 1)
				return nil
			default:
				return cerr.Usage(fmt.Sprintf("unsupported schema format %q", format))
			}
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "json, mcp, markdown")
	return cmd
}

// commandSchema is the serialized shape of a command in the registry.
type commandSchema struct {
	Path  string           `json:"path"`
	Use   string           `json:"use"`
	Short string           `json:"short"`
	Args  string           `json:"args,omitempty"`
	Sub   []*commandSchema `json:"subcommands,omitempty"`
}

func buildSchema(root *cobra.Command) *commandSchema {
	rootSC := &commandSchema{Path: "", Use: root.Use, Short: root.Short}
	nodes := map[*cobra.Command]*commandSchema{root: rootSC}
	WalkCommands(root, func(path []string, c *cobra.Command) {
		if c == root {
			return
		}
		parent := nodes[c.Parent()]
		sc := &commandSchema{Path: parent.Path + " " + parent.Use, Use: c.Use, Short: c.Short}
		nodes[c] = sc
		parent.Sub = append(parent.Sub, sc)
	})
	return rootSC
}

func walkSchema(c *cobra.Command, root *cobra.Command, depth int) {
	WalkCommands(c, func(path []string, sub *cobra.Command) {
		indent := ""
		for i := 0; i < depth+len(path); i++ {
			indent += "  "
		}
		fmt.Fprintf(root.OutOrStdout(), "%s- `%s` - %s\n", indent, sub.CommandPath(), sub.Short)
	})
}
