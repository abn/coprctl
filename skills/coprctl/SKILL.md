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
  - `coprctl apply` - Reconcile a project to match the manifest
  - `coprctl build` - Manage Copr builds
    - `coprctl build cancel` - Cancel a build
    - `coprctl build get` - Show a build
    - `coprctl build list` - List builds for a project
    - `coprctl build submit` - Submit a build
  - `coprctl chroot` - The instance chroot catalog (mock chroots)
    - `coprctl chroot list` - List the chroot catalog
  - `coprctl compat` - Translate a copr-cli invocation to coprctl
  - `coprctl completion` - Generate shell completion scripts
    - `coprctl completion bash` - Generate bash completion
    - `coprctl completion fish` - Generate fish completion
    - `coprctl completion powershell` - Generate powershell completion
    - `coprctl completion zsh` - Generate zsh completion
  - `coprctl detect` - Read-only: infer a project setup from a source repository
  - `coprctl diff` - Show field-level drift between manifest and live project
  - `coprctl doctor` - Diagnose environment issues
  - `coprctl export` - Generate a manifest from a live project
  - `coprctl init` - Scaffold a manifest and create a working Copr project
  - `coprctl integration` - Configure forge webhook integrations
    - `coprctl integration github` - GitHub webhook integration
      - `coprctl integration github enable` - Enable a GitHub webhook for a project
    - `coprctl integration rotate-secret` - Generate a new webhook secret and cache it
    - `coprctl integration url` - Print the Copr webhook URL for a project
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
  - `coprctl sync` - Reconcile the manifest against the source repo and Copr
  - `coprctl try` - Local clean-room preflight build in containers
  - `coprctl validate` - Validate a manifest without any network calls
  - `coprctl version` - Print version and build information
