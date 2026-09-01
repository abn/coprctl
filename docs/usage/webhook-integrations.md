---
type: Guide
title: Webhook integrations
description: Wire a Copr project to forge push and tag events with one command.
status: stable
---

# Webhook integrations

The forge integration configures the forge side of a Copr webhook so pushes
and tags to a repository trigger rebuilds. GitHub and GitLab are supported,
each with one command: `coprctl integration github enable` and
`coprctl integration gitlab enable`.

## Access requirements

The GitHub integration needs a GitHub personal access token (PAT) scoped to
the repositories you manage.

- **Scope**: `admin:repo_hook` (classic PATs), or for fine-grained PATs the
  `Webhooks` read/write permission on the target repositories. GitHub's
  guidance on creating fine-grained tokens is at
  [docs.github.com](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens).
- This is the narrowest scope that works and the only one the tool uses. It
  reads no code and writes no commits.
- A broader token is never needed and is worse to leak; keep the scope tight.
- The token is read at runtime from `GITHUB_TOKEN` or `GH_TOKEN`. It is never
  stored by the tool and never printed.

The GitLab integration needs a GitLab personal access token with the `api`
scope on the groups and projects you manage.

- The token is read at runtime from `GITLAB_TOKEN` and is never stored by the
  tool.
- Self-hosted GitLab: set `GITLAB_API_URL` to the API root of your instance,
  including the `/api/v4` prefix. Without it the tool targets gitlab.com.
  Note that package scoping still matches `gitlab.com` clone URLs, so on a
  self-hosted instance the package-scoped URL may not be selected; the
  receiver URL itself is independent of the clone host.

The tool does not yet validate the scope before acting; a missing scope
surfaces as an error from the forge API when the hook is created or updated.

## Before you begin

1. The Copr project exists and is owned by you (or a group you administer).
2. A webhook secret is known to the tool. The secret is stored in local state
   and is not invented: run `coprctl integration rotate-secret` once if none
   is cached.
3. The packages you want to rebuild on push exist in the project.

## Enable a webhook

For GitHub:

```bash
export GH_TOKEN=<your-token>
coprctl integration rotate-secret OWNER/PROJECT --yes
coprctl integration github enable OWNER/PROJECT \
    --repo OWNER/REPO
```

For GitLab:

```bash
export GITLAB_TOKEN=<your-token>
coprctl integration rotate-secret OWNER/PROJECT --yes
coprctl integration gitlab enable OWNER/PROJECT \
    --repo GROUP/PROJECT
```

The default trigger is **tag-only**: GitHub listens for the `create` event and
GitLab for the `tag push` event, both of which fire when a tag is created.
Copr uses the tag name to rebuild the matching package
(`PKGNAME-VERSION[-RELEASE]`). This means branch pushes do not trigger rebuilds
by default.

The trigger is configurable:

- `--tag-only=false` opts back in to branch pushes (GitHub `push` event,
  GitLab `push_events` toggle).
- GitHub additionally accepts `--events push,create` to override the default
  with an explicit event list. GitLab has no event list; its triggers are the
  `push_events` and `tag_push_events` toggles, so `--events` is not offered.

The command:

- resolves the Copr webhook URL from the project id and the cached secret,
  scoped to the matching SCM package so a bare `v<semver>` tag matches that
  package by name;
- enables auto-rebuild on the relevant packages;
- creates the forge hook, or updates an existing one whose destination still
  carries this project's receiver prefix (forge and project id, so a rotated
  secret is recognized), and never duplicates hooks;
- sends a delivery check: GitHub reads back the ping delivery status, GitLab
  triggers a test hook;
- persists the hook id in local state for later reconciliation.

Enabling a webhook implies the tag pushes should rebuild the package, so
auto-rebuild is turned on for the project's SCM packages automatically. Pass
`--no-auto-rebuild` to opt out and only wire the hook.

Both sides are reported in the result, and the webhook URL is masked unless
`--reveal` is passed.

## Disable a webhook

`coprctl integration disable` removes the forge hook a previous enable created.

```bash
coprctl integration disable OWNER/PROJECT \
    --forge github --repo OWNER/REPO --yes
```

The command verifies before it deletes: it lists the hooks on the target repo
and removes the one whose destination equals the expected Copr webhook URL for
that forge and project (including the package scope, when the enable selected
one). The stored hook id is only a hint, so a stale id never deletes a hook
aimed at a different project or forge. If no hook matches, nothing is deleted
and the command reports the mismatch.

Disabling does **not** rotate the Copr webhook secret. The secret is
project-scoped and shared across every hook, so rotation would break other
consumers. Rotate explicitly with `integration rotate-secret` only when you
intend to break all hooks.

## Custom webhooks

Any service can POST to the custom receiver. The route requires a package
name, so the URL is always package-scoped:

```bash
coprctl integration url OWNER/PROJECT --forge custom --package PKG
```

The output is the URL to configure in the external service. For GitHub and
GitLab the same command prints the standard forge URL, and `--package` selects
the package-scoped form:

```bash
coprctl integration url OWNER/PROJECT
coprctl integration url OWNER/PROJECT --forge gitlab --package PKG
```

`--forge bitbucket` is accepted too; the receiver route is identical in shape,
though there is no enable command for Bitbucket yet.

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