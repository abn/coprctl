---
type: Guide
title: GitHub webhook integration
description: Wire a Copr project to GitHub push and tag events with one command.
status: stable
---

# GitHub webhook integration

The GitHub integration configures the forge side of a Copr webhook so pushes
and tags to a repository trigger rebuilds. It is one command:
`coprctl integration github enable`.

## Access requirements

The integration needs a GitHub personal access token (PAT) scoped to the
repositories you manage.

- **Scope**: `admin:repo_hook` (classic PATs), or for fine-grained PATs the
  `Webhooks` read/write permission on the target repositories. GitHub's
  guidance on creating fine-grained tokens is at
  [docs.github.com](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens).
- This is the narrowest scope that works and the only one the tool uses. It
  reads no code and writes no commits.
- A broader token is never needed and is worse to leak; keep the scope tight.
- The token is read at runtime from `GITHUB_TOKEN` or `GH_TOKEN`. It is never
  stored by the tool and never printed.

The tool does not yet validate the scope before acting; a missing scope
surfaces as a 403 from the GitHub API when the hook is created or updated.

## Before you begin

1. The Copr project exists and is owned by you (or a group you administer).
2. A webhook secret is known to the tool. The secret is stored in local state
   and is not invented: run `coprctl integration rotate-secret` once if none
   is cached.
3. The packages you want to rebuild on push exist in the project.

## Enable a webhook

```bash
export GH_TOKEN=<your-token>
coprctl integration rotate-secret OWNER/PROJECT --yes
coprctl integration github enable OWNER/PROJECT \
    --repo OWNER/REPO
```

The default trigger is **tag-only**: the hook listens for the GitHub `create`
event, which fires when a tag is created. Copr uses the tag name to rebuild the
matching package (`PKGNAME-VERSION[-RELEASE]`). This means branch pushes do not
trigger rebuilds by default.

The trigger is configurable:

- `--tag-only=false` opts back in to branch pushes (GitHub `push` event in
  addition to `create`).
- `--events push,create` overrides the default with an explicit event list.

The command:

- resolves the Copr webhook URL from the project id and the cached secret,
  scoped to the matching SCM package so a bare `v<semver>` tag matches that
  package by name;
- enables auto-rebuild on the relevant packages;
- creates the GitHub hook (or updates an existing one that points at the same
  instance, so it never duplicates hooks);
- sends a ping and reads back the delivery, reporting the HTTP status Copr
  returned;
- persists the hook id in local state for later reconciliation.

Enabling a webhook implies the tag pushes should rebuild the package, so
auto-rebuild is turned on for the project's SCM packages automatically. Pass
`--no-auto-rebuild` to opt out and only wire the hook.

Both sides are reported in the result, and the webhook URL is masked unless
`--reveal` is passed.

## Managing the secret

The Copr webhook secret is a credential: it is never written into a manifest,
never in `export` output, and never printed without `--reveal`.

- `coprctl integration url OWNER/PROJECT` prints the webhook URL with the
  secret masked.
- `coprctl integration rotate-secret OWNER/PROJECT --yes` generates a new
  secret and caches it in local state (mode 0600).

Rotation breaks every existing hook by design. Re-enable the hook after
rotating so the forge side points at the new secret.

## Related

- `coprctl doctor` reports connectivity and credential presence.
- See the architecture page on the agent contract for how this surfaces to
  agents.