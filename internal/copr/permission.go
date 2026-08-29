package copr

import (
	"context"
	"fmt"
	"net/http"
)

// PermissionState is the state of a single permission role for a user.
type PermissionState string

const (
	PermissionNothing  PermissionState = "nothing"
	PermissionRequest  PermissionState = "request"
	PermissionApproved PermissionState = "approved"
)

// Valid reports whether s is one of the supported permission states.
func (s PermissionState) Valid() bool {
	switch s {
	case PermissionNothing, PermissionRequest, PermissionApproved:
		return true
	}
	return false
}

// PermissionSet holds the admin and builder permission states for one user.
type PermissionSet struct {
	Admin   PermissionState `json:"admin"`
	Builder PermissionState `json:"builder"`
}

// Permissions maps a username to its permission set. The GET endpoint wraps
// this map under a "permissions" key; the SET endpoint accepts the bare map as
// the request body.
type Permissions map[string]PermissionSet

// ListPermissions returns the permission map for a project.
func (c *Client) ListPermissions(ctx context.Context, owner, project string) (Permissions, error) {
	var env struct {
		Permissions Permissions `json:"permissions"`
	}
	path := fmt.Sprintf("/project/permissions/get/%s/%s", owner, project)
	if err := c.Get(ctx, path, nil, &env); err != nil {
		return nil, err
	}
	return env.Permissions, nil
}

// SetPermissions replaces the permission set for the named users, returning
// the list of users whose permissions were updated. Roles whose state is empty
// are omitted, so a partial set only touches the roles it names.
func (c *Client) SetPermissions(ctx context.Context, owner, project string, perms Permissions) ([]string, error) {
	var resp struct {
		Updated []string `json:"updated"`
	}
	path := fmt.Sprintf("/project/permissions/set/%s/%s", owner, project)
	if err := c.doJSON(ctx, http.MethodPost, path, permissionSetBody(perms), &resp); err != nil {
		return nil, err
	}
	return resp.Updated, nil
}

// permissionSetBody builds the set request body, keeping only the users and
// roles that carry a state. An empty state would be rejected by the server,
// and omitting it leaves the untouched role unchanged.
func permissionSetBody(perms Permissions) map[string]map[string]PermissionState {
	body := make(map[string]map[string]PermissionState, len(perms))
	for user, set := range perms {
		roles := make(map[string]PermissionState, 2)
		if set.Admin != "" {
			roles["admin"] = set.Admin
		}
		if set.Builder != "" {
			roles["builder"] = set.Builder
		}
		if len(roles) > 0 {
			body[user] = roles
		}
	}
	return body
}

// RequestPermissions requests admin/builder access to a project and reports
// whether the request updated anything.
func (c *Client) RequestPermissions(ctx context.Context, owner, project string, admin, builder bool) (bool, error) {
	payload := map[string]bool{"admin": admin, "builder": builder}
	var resp struct {
		Updated bool `json:"updated"`
	}
	path := fmt.Sprintf("/project/permissions/request/%s/%s", owner, project)
	if err := c.doJSON(ctx, http.MethodPost, path, payload, &resp); err != nil {
		return false, err
	}
	return resp.Updated, nil
}

// CanBuildIn reports whether who can build in the given project.
func (c *Client) CanBuildIn(ctx context.Context, who, owner, project string) (bool, error) {
	var resp struct {
		CanBuildIn bool `json:"can_build_in"`
	}
	path := fmt.Sprintf("/project/permissions/can_build_in/%s/%s/%s", who, owner, project)
	if err := c.Get(ctx, path, nil, &resp); err != nil {
		return false, err
	}
	return resp.CanBuildIn, nil
}
