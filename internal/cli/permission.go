package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/abn/coprctl/internal/cerr"
	"github.com/abn/coprctl/internal/copr"
	"github.com/abn/coprctl/internal/render"
)

func newProjectPermissionCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "permission",
		Short: "Manage project permissions",
	}
	cmd.AddCommand(
		newProjectPermissionListCmd(app, out),
		newProjectPermissionSetCmd(app, out),
		newProjectPermissionRequestCmd(app, out),
		newProjectPermissionCanBuildInCmd(app, out),
	)
	return cmd
}

func newProjectPermissionListCmd(app *App, out *outFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list REF",
		Short: "List the permissions of a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			perms, err := c.ListPermissions(cmd.Context(), r.Owner, r.Project)
			if err != nil {
				return err
			}
			if isHuman(out.format) {
				t := render.NewTable("USER", "ADMIN", "BUILDER")
				for _, user := range render.SortedKeys(perms) {
					t.Add(user, string(perms[user].Admin), string(perms[user].Builder))
				}
				return renderResult(cmd, out, t)
			}
			return renderResult(cmd, out, perms)
		},
	}
	bindRefCompletion(app, cmd)
	return cmd
}

// parsePermissionState validates a --admin/--builder flag value. An empty
// value means the role was not requested and passes through untouched, so a
// partial set does not clobber the role left unset.
func parsePermissionState(role, value string) (copr.PermissionState, error) {
	if value == "" {
		return "", nil
	}
	s := copr.PermissionState(value)
	if !s.Valid() {
		return "", cerr.Usage(fmt.Sprintf("invalid --%s state %q: use nothing, request, or approved", role, value))
	}
	return s, nil
}

// validatePermissionSetFlags enforces the set command's flag contract: a user
// and at least one role are required.
func validatePermissionSetFlags(user, admin, builder string) error {
	if user == "" {
		return cerr.Usage("--user is required")
	}
	if admin == "" && builder == "" {
		return cerr.Usage("at least one of --admin or --builder is required")
	}
	return nil
}

func newProjectPermissionSetCmd(app *App, out *outFlags) *cobra.Command {
	var user, admin, builder string
	cmd := &cobra.Command{
		Use:   "set REF --user NAME [--admin STATE] [--builder STATE]",
		Short: "Set a user's permissions on a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			if err := validatePermissionSetFlags(user, admin, builder); err != nil {
				return err
			}
			adminState, err := parsePermissionState("admin", admin)
			if err != nil {
				return err
			}
			builderState, err := parsePermissionState("builder", builder)
			if err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			updated, err := c.SetPermissions(cmd.Context(), r.Owner, r.Project, copr.Permissions{
				user: {Admin: adminState, Builder: builderState},
			})
			if err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"updated": updated})
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "username to update")
	cmd.Flags().StringVar(&admin, "admin", "", "admin state: nothing, request, approved")
	cmd.Flags().StringVar(&builder, "builder", "", "builder state: nothing, request, approved")
	bindRefCompletion(app, cmd)
	return cmd
}

// validatePermissionRequestFlags enforces the request command's flag contract:
// at least one role must be requested.
func validatePermissionRequestFlags(admin, builder bool) error {
	if !admin && !builder {
		return cerr.Usage("at least one of --admin or --builder is required")
	}
	return nil
}

func newProjectPermissionRequestCmd(app *App, out *outFlags) *cobra.Command {
	var admin, builder bool
	cmd := &cobra.Command{
		Use:   "request REF [--admin] [--builder]",
		Short: "Request permissions on a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			if err := validatePermissionRequestFlags(admin, builder); err != nil {
				return err
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			updated, err := c.RequestPermissions(cmd.Context(), r.Owner, r.Project, admin, builder)
			if err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"updated": updated})
		},
	}
	cmd.Flags().BoolVar(&admin, "admin", false, "request admin access")
	cmd.Flags().BoolVar(&builder, "builder", false, "request builder access")
	bindRefCompletion(app, cmd)
	return cmd
}

func newProjectPermissionCanBuildInCmd(app *App, out *outFlags) *cobra.Command {
	var user string
	cmd := &cobra.Command{
		Use:   "can-build-in REF --user NAME",
		Short: "Check whether a user can build in a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := parseRef(args[0])
			if err != nil {
				return err
			}
			if user == "" {
				return cerr.Usage("--user is required")
			}
			c, err := app.Client()
			if err != nil {
				return err
			}
			can, err := c.CanBuildIn(cmd.Context(), user, r.Owner, r.Project)
			if err != nil {
				return err
			}
			return renderResult(cmd, out, map[string]any{"user": user, "can_build_in": can})
		},
	}
	cmd.Flags().StringVar(&user, "user", "", "username to check")
	bindRefCompletion(app, cmd)
	return cmd
}
