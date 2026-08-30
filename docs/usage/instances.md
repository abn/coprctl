---
type: Guide
title: Instances, staging, and profiles
description: Work with any Copr instance, including Fedora staging, via profiles.
status: stable
---

# Instances, staging, and profiles

coprctl is instance-agnostic. A profile is a named Copr instance: its base URL
plus the credentials you use there. The default profile is the Fedora
production instance.

## Profile names

Well-known public instances get friendly profile names:

- `production` - `copr.fedorainfracloud.org` (the default)
- `staging` - `copr.stg.fedoraproject.org`
- `openEuler` - the openEuler Copr instance

Self-hosted Copr instances are named by their hostname. The profile name is
detected from the instance URL whenever one is needed, so you rarely type it.

## Authenticate to an instance

`auth login --url` opens the instance API page in your browser and imports a
fresh token into a profile named after the instance:

```bash
coprctl auth login --url https://copr.stg.fedoraproject.org
```

This creates a `staging` profile. `--profile` overrides the auto-name, and
`--no-open` prints the URL instead of opening a browser.

To import an existing token from a pasted copr config block:

```bash
coprctl config import --url https://copr.stg.fedoraproject.org <<'EOF'
[copr-cli]
login = "..."
username = "you"
token = "..."
copr_url = "https://copr.stg.fedoraproject.org"
EOF
```

`config show` confirms what is configured and where each value came from:

```bash
coprctl config show
```

## Use a profile

Pass `--profile` to any command:

```bash
coprctl --profile staging project list
coprctl --profile staging project create @group/proj --chroot fedora-rawhide-x86_64
```

The first profile you create becomes the default. Change it by setting
`default_profile` in the config file; `coprctl config show` reports the
effective profile.

## Staging

Staging exists for testing, and its data is wiped periodically. Do not rely on
anything you create there. `coprctl doctor` checks connectivity per profile, so
run it after a staging wipe or a network change:

```bash
coprctl --profile staging doctor
coprctl --profile staging chroot list --distro fedora-rawhide
```

Staging is a separate instance with separate data, so projects and builds you
create there never touch production, and vice versa.

## Related

- The command reference: `coprctl config`, `coprctl auth`, `coprctl doctor`.
- Group namespaces, which work on any instance:
  [Group projects](group-projects.md).