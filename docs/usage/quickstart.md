---
type: Guide
title: Quick start
description: First steps with coprctl: install, authenticate, and run a first build.
status: stable
---

# Quick start

This walkthrough takes you from a fresh install to a running Copr build.

## Install

```bash
go install github.com/abn/coprctl/cmd/coprctl@latest
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

See the [usage index](index.md) for the full guides, and the
[GitHub integration](github-integration.md) guide to wire webhooks.