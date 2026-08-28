---
type: Guide
title: Quick start
description: First steps with coprctl: install, authenticate, and run a first build.
status: stable
---

# Quick start

This walkthrough takes you from a fresh install to a running Copr build.

## Install

The easiest way is from this Copr repository, which keeps the binary updated
with your distro:

```bash
sudo dnf copr enable abn/coprctl
sudo dnf install coprctl
```

Or build from source with the Go toolchain:

```bash
go install github.com/abn/coprctl/cmd/coprctl@latest
```

Verify the setup:

```bash
coprctl auth status
coprctl doctor
```

## Authenticate

Your existing `~/.config/copr` is picked up automatically. For a fresh token,
`auth login` opens the instance API page in your browser:

```bash
coprctl auth login
```

Or import the `[copr-cli]` block the site offers:

```bash
coprctl config import <<'EOF'
[copr-cli]
login = "..."                 # from the website
username = "you"
token = "..."                 # from the website
copr_url = "https://copr.fedorainfracloud.org"
EOF
```

Confirm you are set up:

```bash
coprctl auth status           # who you are, and how long until the token expires
coprctl doctor                # config, auth, and connectivity checks
coprctl chroot list --distro fedora-rawhide
```

## Create a project and build

```bash
coprctl project create you/mypkg --chroot fedora-rawhide-x86_64

# set the project metadata up front
coprctl project create you/mypkg --chroot fedora-rawhide-x86_64 \
  --description "My package" --homepage https://example.org \
  --contact "https://github.com/you/mypkg/issues" \
  --instructions install.md          # a markdown file, or inline text

# link a GitHub repo; homepage and issues contact are derived when unset
coprctl project create you/mypkg --github-repo you/mypkg \
  --instructions install.md

# or edit it later
coprctl project edit you/mypkg --description "..." --homepage ... \
  --contact ... --instructions install.md --github-repo you/mypkg

# from a local spec, let coprctl infer the setup first
coprctl detect ./rpm --output json

# scaffold a project from a source repo
coprctl init --owner you --name mypkg \
  --chroot fedora-44-x86_64 --chroot fedora-rawhide-x86_64 --yes
```

## Watch and debug

```bash
coprctl build rebuild you/mypkg/mypkg
coprctl build watch 10653539
coprctl log failures 10653539
```

## Declarative from here

For repeatable projects, put the state in a `copr.yaml` manifest and reconcile:

```bash
coprctl export you/mypkg -o copr.yaml     # adopt an existing project
coprctl validate -f copr.yaml
coprctl apply -f copr.yaml
```

## Chroot lifecycle

`chroot list` shows the instance catalog with each chroot's EOL state, so you
can see at a glance which releases are retired:

```bash
coprctl chroot list --state active      # only current releases
coprctl chroot list --distro fedora
```

Targeting an EOL chroot for a build warns that it will not accept new builds.

To add or remove chroots on a project:

```bash
coprctl project chroot enable you/mypkg --chroot fedora-45-x86_64
coprctl project chroot disable you/mypkg --chroot fedora-42-x86_64 --yes
```

Or reconcile chroots from a manifest, pruning those the manifest no longer
lists:

```bash
coprctl apply -f copr.yaml --prune --yes
```

See the [usage index](index.md) for the full guides, and the
[GitHub integration](github-integration.md) guide to wire webhooks.