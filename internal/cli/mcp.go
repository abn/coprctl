package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/mcp"
)

// mcpRegistry exposes the command tree as MCP tools, walking to leaf verbs and
// tiering them by destructiveness.
type mcpRegistry struct {
	root *cobra.Command
}

// destructiveVerbs are leaf command names whose effect is destructive and thus
// require the destructive tier.
var destructiveVerbs = map[string]bool{
	"delete": true, "cancel": true, "rotate-secret": true, "fork": true,
}

func (m *mcpRegistry) Tools(tier string) []mcp.Tool {
	var tools []mcp.Tool
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		subs := c.Commands()
		if len(subs) == 0 {
			// Leaf verb.
			name := "coprctl_" + strings.Join(path, "_")
			tierName := "read"
			for _, p := range path {
				if destructiveVerbs[p] {
					tierName = "destructive"
					break
				}
			}
			if tier != "destructive" && tierName == "destructive" {
				return
			}
			if tier == "read" && tierName != "read" {
				return
			}
			tools = append(tools, mcp.Tool{
				Name:        name,
				Description: c.Short,
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
				Tier:        tierName,
			})
			return
		}
		for _, sub := range subs {
			if !sub.IsAvailableCommand() {
				continue
			}
			walk(sub, append(path, sub.Name()))
		}
	}
	for _, sub := range m.root.Commands() {
		if sub.IsAvailableCommand() {
			walk(sub, []string{sub.Name()})
		}
	}
	return tools
}

// Call executes a leaf command by name with string arguments and returns its
// stdout. It runs the real command so the MCP surface cannot drift from the
// CLI.
func (m *mcpRegistry) Call(app *App, toolName string, args []string) (string, error) {
	cmd := findLeaf(m.root, toolName)
	if cmd == nil {
		return "", fmt.Errorf("unknown tool %q", toolName)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if cmd.RunE == nil {
		return "", fmt.Errorf("tool %q has no runnable implementation", toolName)
	}
	err := cmd.RunE(cmd, args)
	return out.String(), err
}

// findLeaf locates a leaf command by its MCP tool name suffix.
func findLeaf(root *cobra.Command, toolName string) *cobra.Command {
	// toolName is like coprctl_project_get. Split off the prefix and match by
	// the joined path.
	parts := strings.Split(toolName, "_")
	if len(parts) < 2 || parts[0] != "coprctl" {
		return nil
	}
	path := parts[1:]
	var walk func(c *cobra.Command, remaining []string) *cobra.Command
	walk = func(c *cobra.Command, remaining []string) *cobra.Command {
		if len(remaining) == 0 {
			return c
		}
		for _, sub := range c.Commands() {
			if sub.Name() == remaining[0] && sub.IsAvailableCommand() {
				if found := walk(sub, remaining[1:]); found != nil {
					return found
				}
			}
		}
		return nil
	}
	return walk(root, path)
}

func newMCPServeCmd(app *App) *cobra.Command {
	var allowWrite, allowDestructive bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve the command surface as MCP tools over stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			tier := "read"
			if allowDestructive {
				tier = "destructive"
			} else if allowWrite {
				tier = "write"
			}
			srv := &mcp.Server{
				In:   os.Stdin,
				Out:  os.Stdout,
				Reg:  &mcpRegistry{root: Root(app)},
				Tier: tier,
				Call: func(name string, args []string) (string, error) {
					return (&mcpRegistry{root: Root(app)}).Call(app, name, args)
				},
			}
			return srv.Serve()
		},
	}
	cmd.Flags().BoolVar(&allowWrite, "allow-write", false, "enable write-tier tools")
	cmd.Flags().BoolVar(&allowDestructive, "allow-destructive", false, "enable destructive-tier tools")
	return cmd
}

func newMCPCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server",
	}
	cmd.AddCommand(newMCPServeCmd(app))
	return cmd
}

// schemaMCP builds the MCP tool definitions from the registry.
func schemaMCP(root *cobra.Command) []mcp.Tool {
	return (&mcpRegistry{root: root}).Tools("destructive")
}
