---
type: Guide
title: Group projects
description: Own a Copr project as a team with group namespaces (@alias).
status: stable
---

# Group projects

A Copr group is a shared namespace owned by an `@alias` instead of a single
user. Projects under a group are owned collectively: every member of the Fedora
Account System (FAS) group behind the alias can build in them and edit them
without any per-user permission rows.

Membership comes from FAS and is refreshed on login. Adding or removing a member
is a FAS admin action, not a Copr one.

## Setting up a group

Creating a group is a one-time step that the Copr API does not expose, so it
happens in the web UI. Until it completes, the `@alias` does not exist and any
command that references it fails.

1. Create the FAS group. A brand-new FAS group needs a Fedora infrastructure
   ticket; an existing FAS group can be reused as-is.
2. Log out of Copr and log back in so Copr picks up the new FAS membership.
3. Activate the group at `<instance>/groups/list/my` (the UI POSTs to
   `/groups/activate/<fas_group>` with the alias you choose). The alias is what
   appears as `@alias` in references.

There is no group-activation command in `coprctl`; the web UI is the only place
this step exists.

## Using a group with coprctl

Every project-family command accepts an `@alias/project` reference:

```bash
# create a project under the group, once the group is activated
coprctl project create @coprctl/coprctl --chroot fedora-rawhide-x86_64

# list the group's projects
coprctl project list @coprctl

# build into a group project from a local spec directory
coprctl build submit @coprctl/coprctl --from ./specs/mypkg

# or rebuild a package from its stored source definition
coprctl build rebuild @coprctl/coprctl/mypkg

# watch and inspect builds
coprctl build list @coprctl/coprctl
```

FAS members get build and edit rights automatically, so a team working out of
one group needs no additional permissions.

## Permissions for non-members

`project permission` works on group projects to grant builder and admin roles
to people outside the FAS group:

```bash
coprctl project permission list @coprctl/coprctl
coprctl project permission set @coprctl/coprctl --user you --builder approved
coprctl project permission can-build-in @coprctl/coprctl --user you
```

Roles accept `nothing`, `request`, or `approved`; an empty value leaves that
role untouched.

## When the group is not activated

If the group was never activated, commands that resolve an `@alias` owner fail
with a 404 and an activation hint:

```
group "@coprctl" is not activated; activate it at
https://copr.fedorainfracloud.org/groups/list/my first
```

Activate the group in the web UI (logging out and back in first if the FAS
group is new) and rerun the command. No local state is involved.

## What coprctl cannot do

- Activating a group.
- Listing "my groups".

Both are web-UI-only upstream, with no `/api_3` endpoint behind them, so
`coprctl` has no commands for either. Use `<instance>/groups/list/my` in the
browser.

## Related

- The command reference for the project family: `coprctl project ...`.
- Profiles for working against other instances, including staging:
  [Instances, staging, and profiles](instances.md).