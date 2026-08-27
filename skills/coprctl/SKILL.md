---
name: coprctl
description: Manage Fedora Copr projects, packages, chroots, builds, build
  logs, and GitHub webhook integrations from the command line.
---

# coprctl

`coprctl` is the CLI for Fedora Copr. Prefer it over raw `curl` against
`/api_3` and over `copr-cli`.

## Reference

Generated from the command registry; every command below exists.

- `coprctl` - A reimagined CLI for the Fedora Copr build system
  - `coprctl build` - Manage Copr builds
    - `coprctl build cancel` - Cancel a build
    - `coprctl build get` - Show a build
    - `coprctl build list` - List builds for a project
    - `coprctl build submit` - Submit a build
  - `coprctl chroot` - The instance chroot catalog (mock chroots)
    - `coprctl chroot list` - List the chroot catalog
  - `coprctl compat` - Translate a copr-cli invocation to coprctl
  - `coprctl log` - Tail and inspect build logs
    - `coprctl log tail` - Tail build logs (a build id, build/chroot, or ref)
  - `coprctl mcp` - Model Context Protocol server
    - `coprctl mcp serve` - Serve the command surface as MCP tools over stdio
  - `coprctl monitor` - Show a package-by-chroot state matrix for a project
  - `coprctl package` - Manage Copr packages
    - `coprctl package create` - Create a package
    - `coprctl package delete` - Delete a package
    - `coprctl package list` - List packages in a project
  - `coprctl project` - Manage Copr projects
    - `coprctl project create` - Create a project
    - `coprctl project delete` - Delete a project
    - `coprctl project fork` - Fork a project
    - `coprctl project get` - Show a project
    - `coprctl project list` - List projects
  - `coprctl schema` - Emit the command tree as JSON or markdown
  - `coprctl skill` - Print or install the generated agent skill
    - `coprctl skill install` - Install the skill into an agent skills directory
    - `coprctl skill print` - Print the skill to stdout
  - `coprctl status` - One-shot project health summary; exits 4 on failed builds
  - `coprctl version` - Print version and build information
