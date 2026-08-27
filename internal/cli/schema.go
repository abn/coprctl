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
	return walk(root, "")
}

func walk(c *cobra.Command, prefix string) *commandSchema {
	path := prefix + " " + c.Use
	sc := &commandSchema{Path: prefix, Use: c.Use, Short: c.Short}
	for _, sub := range c.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		sc.Sub = append(sc.Sub, walk(sub, path))
	}
	return sc
}

func walkSchema(c *cobra.Command, root *cobra.Command, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}
	fmt.Fprintf(root.OutOrStdout(), "%s- `%s` - %s\n", indent, c.CommandPath(), c.Short)
	for _, sub := range c.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		walkSchema(sub, root, depth+1)
	}
}
