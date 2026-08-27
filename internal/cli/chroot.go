package cli

import (
	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/render"
	"github.com/abn/coprctl/internal/state"
)

func newChrootCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "chroot",
		Short: "The instance chroot catalog (mock chroots)",
	}
	out.bind(cmd)
	cmd.AddCommand(newChrootListCmd(app, &out))
	return cmd
}

func newChrootListCmd(app *App, out *outFlags) *cobra.Command {
	var distro, arch, filter string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the chroot catalog",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := app.Client()
			if err != nil {
				return err
			}
			chroots, err := c.ListMockChroots(cmd.Context())
			if err != nil {
				return err
			}
			// Cache the catalog for offline reference disambiguation.
			if cacheDir, err := state.CacheDir(app.profile); err == nil {
				_ = state.NewChrootCache(cacheDir).Store(chroots)
			}
			names := filterChroots(chroots.ChrootNames(), filter, distro, arch)
			if isHuman(out.format) {
				t := render.NewTable("CHROOT", "COMMENT")
				for _, n := range names {
					t.Add(n, chroots[n])
				}
				return renderResult(cmd, out, t)
			}
			filtered := copr.MockChroots{}
			for _, n := range names {
				filtered[n] = chroots[n]
			}
			return renderResult(cmd, out, filtered)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "glob filter on chroot name")
	cmd.Flags().StringVar(&distro, "distro", "", "distro filter")
	cmd.Flags().StringVar(&arch, "arch", "", "arch filter")
	return cmd
}
