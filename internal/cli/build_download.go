package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/ref"
	"github.com/abn/coprctl/internal/render"
)

func newBuildDownloadCmd(app *App, out *outFlags) *cobra.Command {
	var chroots []string
	var dest string
	var rpms, logs, spec bool
	cmd := &cobra.Command{
		Use:   "download BUILD_ID",
		Short: "Download a build's artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := ref.Parse(args[0], nil)
			if err != nil {
				return err
			}
			if r.Kind != ref.KindBuild {
				return fmt.Errorf("expected a build id, got %q", args[0])
			}
			c, err := app.ReadClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			build, err := c.GetBuild(ctx, r.BuildID)
			if err != nil {
				return err
			}
			bp, err := c.GetBuiltPackages(ctx, r.BuildID)
			if err != nil {
				return err
			}
			bchroots, err := c.ListBuildChroots(ctx, r.BuildID)
			if err != nil {
				return err
			}
			resultURLs := map[string]string{}
			for _, bc := range bchroots {
				resultURLs[bc.Chroot] = bc.ResultURL
			}
			names := filterBuildChroots(bp, chroots)
			perChroot := map[string][]string{}
			downloaded := 0
			for _, name := range names {
				if resultURLs[name] == "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "no result url for build %d chroot %s; skipping\n", r.BuildID, name)
					continue
				}
				dir := filepath.Join(dest, name)
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return err
				}
				var files []string
				if rpms {
					for _, p := range bp[name].Packages {
						fn := copr.BuiltPackageFilename(p)
						if err := downloadArtifact(cmd, c, ctx, resultURLs[name], fn, dir); err != nil {
							return err
						}
						files = append(files, fn)
						downloaded++
					}
				}
				if logs {
					if err := downloadArtifact(cmd, c, ctx, resultURLs[name], "builder-live.log.gz", dir); err != nil {
						return err
					}
					files = append(files, "builder-live.log.gz")
					downloaded++
				}
				if spec {
					fn := build.PackageName + ".spec"
					if err := downloadArtifact(cmd, c, ctx, resultURLs[name], fn, dir); err != nil {
						return err
					}
					files = append(files, fn)
					downloaded++
				}
				if len(files) > 0 {
					perChroot[name] = files
				}
			}
			if isHuman(out.format) {
				t := render.NewTable("CHROOT", "FILES")
				for _, name := range names {
					if len(perChroot[name]) == 0 {
						continue
					}
					t.Add(name, strings.Join(perChroot[name], ", "))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, map[string]any{
				"build_id":   r.BuildID,
				"chroots":    names,
				"downloaded": downloaded,
				"dest":       dest,
			})
		},
	}
	out.bind(cmd)
	cmd.Flags().StringSliceVar(&chroots, "chroot", nil, "chroots to download (globs allowed, repeatable)")
	cmd.Flags().StringVar(&dest, "dest", ".", "destination directory")
	cmd.Flags().BoolVar(&rpms, "rpms", true, "download built RPMs")
	cmd.Flags().BoolVar(&logs, "logs", false, "download builder-live.log.gz")
	cmd.Flags().BoolVar(&spec, "spec", false, "download the package spec")
	bindChrootCompletion(app, cmd, "chroot")
	return cmd
}

// filterBuildChroots returns the sorted chroot names in bp matching any of the
// given globs, or all of them when no glob is given.
func filterBuildChroots(bp copr.BuiltPackages, globs []string) []string {
	var names []string
	for name := range bp {
		if len(globs) == 0 || matchesAnyGlob(globs, name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func matchesAnyGlob(globs []string, name string) bool {
	for _, g := range globs {
		if globMatch(g, name) {
			return true
		}
	}
	return false
}

// downloadArtifact fetches resultURL+filename into dir, warning instead of
// failing when the artifact is missing (404).
func downloadArtifact(cmd *cobra.Command, c *copr.Client, ctx context.Context, resultURL, filename, dir string) error {
	url := strings.TrimRight(resultURL, "/") + "/" + filename
	if err := c.DownloadFile(ctx, url, filepath.Join(dir, filename)); err != nil {
		if errors.Is(err, copr.ErrFileNotFound) {
			fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s not found (404); skipping\n", url)
			return nil
		}
		return err
	}
	return nil
}
