package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/render"
	"github.com/abn/coprctl/internal/rpmfile"
)

// rpmListFiles is the machine-readable output of rpm list-files.
type rpmListFiles struct {
	Name    string             `json:"name"`
	Version string             `json:"version"`
	Release string             `json:"release"`
	Arch    string             `json:"arch"`
	Summary string             `json:"summary,omitempty"`
	Files   []rpmfile.FileInfo `json:"files"`
}

func newRpmCmd(app *App) *cobra.Command {
	var out outFlags
	cmd := &cobra.Command{
		Use:   "rpm",
		Short: "Inspect local RPM packages",
	}
	out.bind(cmd)
	cmd.AddCommand(newRpmListFilesCmd(&out))
	return cmd
}

func newRpmListFilesCmd(out *outFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list-files PATH",
		Short: "List the files an RPM package installs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := rpmfile.Open(args[0])
			if err != nil {
				return err
			}
			v := rpmListFiles{
				Name:    r.Name,
				Version: r.Version,
				Release: r.Release,
				Arch:    r.Arch,
				Summary: r.Summary,
				Files:   r.Files,
			}
			return renderHumanOr(cmd, out, v, func() *render.Table {
				t := render.NewTable("MODE", "SIZE", "PATH")
				for _, f := range r.Files {
					t.Add(f.Mode.String(), fmt.Sprintf("%d", f.Size), f.Name)
				}
				return t
			})
		},
	}
}
