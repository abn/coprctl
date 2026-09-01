package cli

import (
	"github.com/spf13/cobra"
)

// yesHelp is the standard help text for the --yes confirmation flag.
const yesHelp = "assume yes for confirmation"

// addYesFlag binds the --yes confirmation flag. With withShort set it also
// binds the -y shorthand. It returns a pointer to read at the gate.
func addYesFlag(cmd *cobra.Command, help string, withShort bool) *bool {
	var yes bool
	if withShort {
		cmd.Flags().BoolVarP(&yes, "yes", "y", false, help)
	} else {
		cmd.Flags().BoolVar(&yes, "yes", false, help)
	}
	return &yes
}

// addChrootFlag binds the --chroot flag, wiring completion from the cached
// chroot catalog. With withShort set it also binds the -r shorthand. It
// returns a pointer to read at the gate.
func addChrootFlag(app *App, cmd *cobra.Command, help string, withShort bool) *[]string {
	var chroots []string
	if withShort {
		cmd.Flags().StringSliceVarP(&chroots, "chroot", "r", nil, help)
	} else {
		cmd.Flags().StringSliceVar(&chroots, "chroot", nil, help)
	}
	bindChrootCompletion(app, cmd, "chroot")
	return &chroots
}

// addExcludeChrootFlag binds the repeatable --exclude-chroot flag, wired to
// the chroot catalog completion like --chroot.
func addExcludeChrootFlag(app *App, cmd *cobra.Command) *[]string {
	var chroots []string
	cmd.Flags().StringSliceVar(&chroots, "exclude-chroot", nil, "chroots to exclude from the build (globs allowed)")
	bindChrootCompletion(app, cmd, "exclude-chroot")
	return &chroots
}

// addRuntimeFlag binds the standard --runtime flag and returns a pointer to
// read at the gate.
func addRuntimeFlag(cmd *cobra.Command) *string {
	var runtimeName string
	cmd.Flags().StringVar(&runtimeName, "runtime", "auto", "build backend: auto, container, native, mock")
	return &runtimeName
}
