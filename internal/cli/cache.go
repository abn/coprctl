package cli

import (
	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/state"
)

func newCacheCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage local caches",
	}
	out.bind(cmd)
	cmd.AddCommand(newCacheClearCmd(app, &out))
	return cmd
}

func newCacheClearCmd(app *App, out *outFlags) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear the local chroot catalog cache",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return confirmRequired("--yes")
			}
			cacheDir, err := state.CacheDir(app.profile)
			if err != nil {
				return err
			}
			if err := state.NewChrootCache(cacheDir).Clear(); err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"cleared": true, "dir": cacheDir})
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "assume yes for confirmation")
	return cmd
}
