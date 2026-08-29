package copr

import (
	"context"
)

// GetProjectChrootBuildConfig fetches the generated mock build config for a
// project chroot. The response is the full buildroot configuration dict as the
// server renders it for a build (the same payload copr-cli mock-config prints).
func (c *Client) GetProjectChrootBuildConfig(ctx context.Context, owner, project, chroot string) (map[string]any, error) {
	q := projectQuery(owner, project)
	q.Set("chrootname", chroot)
	var cfg map[string]any
	if err := c.Get(ctx, "/project-chroot/build-config", q, &cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
