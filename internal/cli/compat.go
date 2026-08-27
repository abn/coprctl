package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// migrationTable maps copr-cli verbs to their coprctl equivalent. Values may
// contain positional placeholders {0}, {1} referencing the original args.
var migrationTable = map[string]string{
	"list":                 "project list",
	"create":               "project create",
	"modify":               "project edit",
	"delete":               "project delete --yes",
	"fork":                 "project fork",
	"regenerate-repos":     "project regenerate-repos",
	"build":                "build submit",
	"buildscm":             "build submit --source scm",
	"buildpypi":            "build submit --source pypi",
	"buildgem":             "build submit --source rubygems",
	"build-distgit":        "build submit --source distgit",
	"buildcustom":          "build submit --source custom",
	"uploadrpm":            "build submit --source upload",
	"build-package":        "build rebuild",
	"status":               "build get",
	"watch-build":          "build watch",
	"cancel":               "build cancel",
	"delete-build":         "build delete --yes",
	"download-build":       "build download",
	"list-builds":          "build list",
	"add-package-scm":      "package create --source scm",
	"edit-package-scm":     "package edit --source scm",
	"add-package-pypi":     "package create --source pypi",
	"edit-package-pypi":    "package edit --source pypi",
	"add-package-gem":      "package create --source rubygems",
	"edit-package-gem":     "package edit --source rubygems",
	"add-package-distgit":  "package create --source distgit",
	"edit-package-distgit": "package edit --source distgit",
	"add-package-custom":   "package create --source custom",
	"edit-package-custom":  "package edit --source custom",
	"list-packages":        "package list",
	"list-package-names":   "package names",
	"get-package":          "package get",
	"delete-package":       "package delete --yes",
	"reset-package":        "package reset --yes",
	"list-chroots":         "chroot list",
	"get-chroot":           "project chroot get",
	"edit-chroot":          "project chroot edit",
	"mock-config":          "mock-config",
	"monitor":              "monitor",
	"edit-permissions":     "project permission grant",
	"list-permissions":     "project permission list",
	"request-permissions":  "project permission request",
	"new-webhook-secret":   "integration rotate-secret",
	"new-api-token":        "auth token rotate",
	"whoami":               "auth whoami",
}

// migrateArgs translates a copr-cli invocation into a coprctl invocation.
// It returns the new argv (without the leading "coprctl").
func migrateArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("no copr-cli verb given")
	}
	verb := args[0]
	target, ok := migrationTable[verb]
	if !ok {
		return "", fmt.Errorf("no migration known for copr-cli verb %q", verb)
	}
	rest := args[1:]
	return strings.TrimSpace(target + " " + strings.Join(rest, " ")), nil
}

func newCompatCmd(app *App) *cobra.Command {
	var run bool
	cmd := &cobra.Command{
		Use:   "compat copr-cli -- <args>",
		Short: "Translate a copr-cli invocation to coprctl",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("usage: coprctl compat copr-cli -- <copr-cli args>")
			}
			// Accept both "compat copr-cli -- buildscm ..." and
			// "compat -- buildscm ...".
			if args[0] == "copr-cli" {
				args = args[1:]
			}
			if len(args) == 0 {
				return fmt.Errorf("usage: coprctl compat copr-cli -- <copr-cli args>")
			}
			translated, err := migrateArgs(args)
			if err != nil {
				return err
			}
			if run {
				return runTranslated(cmd, app, translated)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "coprctl "+translated)
			return nil
		},
	}
	cmd.Flags().BoolVar(&run, "run", false, "execute the translated command")
	return cmd
}

func runTranslated(cmd *cobra.Command, app *App, invocation string) error {
	return fmt.Errorf("--run execution not yet wired; translated command: coprctl %s", invocation)
}
