package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newProjectChrootGetCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get REF/CHROOT",
		Short: "Show the generated build config for a project chroot",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], &ref.Options{ForceChroot: true})
			if err != nil {
				return err
			}
			if r.Kind != ref.KindProjectChroot {
				return fmt.Errorf("expected a project chroot reference, got %q", args[0])
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			cfg, err := c.GetProjectChrootBuildConfig(cmd.Context(), r.Owner, r.Project, r.Segment)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("FIELD", "VALUE")
				for _, k := range render.SortedKeys(cfg) {
					t.Add(k, fmt.Sprintf("%v", cfg[k]))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, cfg)
		},
	}
	return cmd
}
