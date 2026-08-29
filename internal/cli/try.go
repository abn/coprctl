package cli

import (
	"fmt"
	"os"
	"path/filepath"
	gort "runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/render"
)

// rpmbuilderRegistry is the container registry for preflight images.
const rpmbuilderRegistry = "quay.io/abn/rpmbuilder"

// imageMatch describes how a Copr chroot resolves to a preflight image.
type imageMatch struct {
	Chroot     string `json:"chroot"`
	Image      string `json:"image,omitempty"`
	Match      string `json:"match"` // exact | substitute | none
	Reason     string `json:"reason,omitempty"`
	Confidence string `json:"confidence,omitempty"`
}

// resolveChrootImage maps a Copr chroot to an rpmbuilder image tag.
func resolveChrootImage(chroot string) imageMatch {
	parts := strings.SplitN(chroot, "-", 3)
	if len(parts) < 2 {
		return imageMatch{Chroot: chroot, Match: "none", Reason: "unparseable chroot name"}
	}
	distro, version := parts[0], parts[1]

	switch {
	case distro == "fedora":
		return imageMatch{Chroot: chroot, Image: rpmbuilderRegistry + ":fedora-" + version,
			Match: "exact", Confidence: "medium"}
	case distro == "epel":
		clone := "rockylinux-" + version
		return imageMatch{Chroot: chroot, Image: rpmbuilderRegistry + ":" + clone,
			Match: "substitute", Reason: "EPEL has no rpmbuilder tag; uses RHEL clone + epel-release",
			Confidence: "low"}
	case strings.HasPrefix(chroot, "centos-stream-"):
		// SplitN with 3 parts gives ["centos", "stream", "10-x86_64"]; recover
		// the version from the remainder.
		rem := parts[2]
		ver := rem
		if i := strings.Index(rem, "-"); i > 0 {
			ver = rem[:i]
		}
		clone := "rockylinux-" + ver
		return imageMatch{Chroot: chroot, Image: rpmbuilderRegistry + ":" + clone,
			Match: "substitute", Reason: "Stream leads the rebuild; divergence is real", Confidence: "low"}
	default:
		return imageMatch{Chroot: chroot, Match: "none", Reason: "no rpmbuilder tag"}
	}
}

// fidelityGap lists what a container preflight does not reproduce, per spec
// 22.9. It is a required part of the output.
var fidelityGap = []string{
	"Mock buildroot minimalism",
	"enable_net=off",
	"bootstrap and isolation settings",
}

func newTryCmd(app *App) *cobra.Command {
	var out outFlags
	var path string
	var chroots []string
	var runtimeName string
	var emulate, matchSubstitute, requireFullCoverage bool
	cmd := &cobra.Command{
		Use:   "try [REF|PATH]",
		Short: "Local preflight build (container, mock, or native)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := resolveBuilder(runtimeName, "preflight")
			if err != nil {
				return err
			}
			srcPath := "."
			if len(args) == 1 {
				if _, statErr := os.Stat(args[0]); statErr == nil {
					srcPath = args[0]
				} else {
					return fmt.Errorf("path %q not found; pass a local spec directory", args[0])
				}
			}
			if path != "" {
				srcPath = path
			}
			spec, err := findSpec(srcPath)
			if err != nil {
				return err
			}
			targetChroots := chroots
			if len(targetChroots) == 0 {
				targetChroots = []string{"fedora-rawhide-x86_64"}
			}

			var results []map[string]any
			matched := 0
			uncovered := []string{}
			container := b.Name() == "container"
			for _, ch := range targetChroots {
				// Container preflight gates on image match, substitution, and
				// host arch. Native and mock backends run the chroot directly.
				if container {
					m := resolveChrootImage(ch)
					if m.Match == "none" {
						uncovered = append(uncovered, ch)
						results = append(results, map[string]any{
							"chroot": ch, "match": "none", "reason": m.Reason,
						})
						continue
					}
					// Strict match is the default: substitutions require opt-in.
					if m.Match == "substitute" && !matchSubstitute {
						uncovered = append(uncovered, ch)
						results = append(results, map[string]any{
							"chroot": ch, "match": "substitute", "status": "skipped",
							"reason": m.Reason + "; pass --match substitute to allow",
						})
						continue
					}
					matched++
					if !emulate && !sameArch(ch) {
						uncovered = append(uncovered, ch)
						m.Confidence = "low"
						results = append(results, map[string]any{
							"chroot": ch, "image": m.Image, "match": m.Match,
							"status": "skipped", "reason": "arch mismatch", "confidence": m.Confidence,
						})
						continue
					}
					// Run the two-stage preflight.
					var status string
					if err := b.Preflight(cmd.Context(), spec, ch, cmd.OutOrStdout()); err != nil {
						status = "failed"
					} else {
						status = "passed"
					}
					results = append(results, map[string]any{
						"chroot": ch, "image": m.Image, "match": m.Match,
						"status": status, "confidence": m.Confidence,
						"backend": "container",
					})
					continue
				}
				matched++
				var status string
				if err := b.Preflight(cmd.Context(), spec, ch, cmd.OutOrStdout()); err != nil {
					status = "failed"
				} else {
					status = "passed"
				}
				results = append(results, map[string]any{
					"chroot": ch, "match": "native", "status": status,
					"confidence": "low", "backend": b.Name(),
				})
			}

			// Fidelity + coverage report (required by spec 22.9).
			report := map[string]any{
				"chroots":          results,
				"coverage":         fmt.Sprintf("%d of %d chroots have a local image", matched, len(targetChroots)),
				"not_reproduced":   fidelityGap,
				"filter_not_proof": true,
			}
			if len(uncovered) > 0 {
				report["uncovered"] = uncovered
			}
			if isHuman(out.format) {
				t := render.NewTable("CHROOT", "STATUS", "MATCH", "CONFIDENCE")
				for _, r := range results {
					t.Add(fmt.Sprintf("%v", r["chroot"]), fmt.Sprintf("%v", r["status"]),
						fmt.Sprintf("%v", r["match"]), fmt.Sprintf("%v", r["confidence"]))
				}
				if err := renderResult(cmd, &out, t); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s\nNot reproduced locally: %s\n",
					report["coverage"], strings.Join(fidelityGap, "; "))
			} else {
				if err := renderResult(cmd, &out, report); err != nil {
					return err
				}
			}

			// Exit code: 4 if any matched build failed; 12 if full coverage
			// was required but not met.
			for _, r := range results {
				if r["status"] == "failed" {
					return cerr.New("preflight_failed", cerr.ExitBuildFailed, "one or more local preflight builds failed")
				}
			}
			if requireFullCoverage && len(uncovered) > 0 {
				return cerr.New("coverage_required", cerr.ExitDrift,
					fmt.Sprintf("full coverage required but %d chroots are uncovered", len(uncovered)))
			}
			return nil
		},
	}
	out.bind(cmd)
	cmd.Flags().StringVar(&path, "path", "", "path to the spec directory")
	cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, "chroots to build (default fedora-rawhide-x86_64)")
	cmd.Flags().StringVar(&runtimeName, "runtime", "auto", "build backend: auto, container, native, mock")
	bindChrootCompletion(app, cmd, "chroot")
	cmd.Flags().BoolVar(&emulate, "emulate", false, "allow emulated (non-host) architectures")
	cmd.Flags().BoolVar(&matchSubstitute, "match", false, "allow substitute images (strict by default)")
	cmd.Flags().BoolVar(&requireFullCoverage, "require-full-coverage", false, "exit 12 if any chroot is uncovered")
	return cmd
}

func findSpec(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".spec") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .spec file found in %s", dir)
}

// findSRPM locates the most recently produced source RPM under a directory.
// The container SRPM_ONLY build writes into <workdir>/.rpmbuild/, and native
// rpmbuild writes into <workdir>/.rpmbuild/SRPMS/, so both are searched.
func findSRPM(dir string) (string, error) {
	candidates := []string{dir, filepath.Join(dir, ".rpmbuild"), filepath.Join(dir, ".rpmbuild", "SRPMS")}
	var best string
	var bestMod int64
	for _, d := range candidates {
		entries, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".src.rpm") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Unix() > bestMod {
				bestMod = info.ModTime().Unix()
				best = filepath.Join(d, e.Name())
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("no .src.rpm found in %s (did the container build produce one?)", dir)
	}
	return best, nil
}

// sameArch reports whether the chroot's architecture matches the host. Copr
// arch names differ from Go GOARCH values (x86_64 vs amd64, aarch64 vs arm64).
func sameArch(chroot string) bool {
	parts := strings.SplitN(chroot, "-", 3)
	if len(parts) < 3 {
		return true // unknown arch: assume compatible
	}
	return normalizeArch(parts[2]) == gort.GOARCH
}

func normalizeArch(arch string) string {
	switch arch {
	case "x86_64":
		return "amd64"
	case "aarch64":
		return "arm64"
	case "i386":
		return "386"
	}
	return arch
}
