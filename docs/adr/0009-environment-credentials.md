---
type: Decision
title: ADR 0009 - Environment credentials
description: Ephemeral credentials from environment variables take precedence over the config file and are only persisted by auth login.
status: stable
---

## Status

Stable.

## Context

Credentials live in the coprctl TOML config or the legacy `~/.config/copr`
file. CI pipelines, containers, and one-off shells need to supply credentials
without writing a file first, and without leaving a token behind afterwards.

Two facts constrain the design. First, Copr API v3 authenticates with HTTP
Basic `login:token`; the server looks the account up by the API login before it
compares the token, so a token on its own can never be accepted. Second, the
account username (the owner used to expand a bare project reference) is a
separate field from the API login and is only known to the server.

Upstream copr-cli reads no environment variables at all. The convention that
exists is a CI habit: `COPR_LOGIN`, `COPR_USERNAME`, and `COPR_TOKEN` are
common secret names, and `COPR_CONFIG` often holds the entire `[copr-cli]`
block, because that is the text the Copr API page offers.

## Decision

Recognise these variables, with the `COPRCTL_` name winning when both forms are
set:

| Variable | Meaning |
| --- | --- |
| `COPRCTL_CONFIG` | a verbatim `[copr-cli]` credential block |
| `COPRCTL_TOKEN` | the API token |
| `COPRCTL_LOGIN` | the API login, required whenever a token is set |
| `COPRCTL_USERNAME` | the account username, when it should not be resolved live |
| `COPRCTL_URL` | the instance base URL, defaulting to production |

Environment credentials always take precedence over the file configuration and
are ephemeral: no command writes them. `auth login` is the single path that
persists them, importing the same block it would otherwise read from a paste.
`auth rotate` refuses to rotate environment credentials unless `--reveal` is
passed, because the replacement token would have nowhere to persist.

The username resolves from `COPRCTL_USERNAME`, then the block, then a live
`auth-check` lookup that is cached after the first success. The
instance resolves from the variable, then the block, and otherwise defaults to
production; environment credentials never inherit the file profile's URL, so a
token cannot be redirected to an unrelated host by a stale profile.

A token without a login is a hard error. So is a config block with no token.
Neither degrades silently to an anonymous client.

## Consequences

- A pipeline can run `COPRCTL_TOKEN=... COPRCTL_LOGIN=... coprctl build submit`
  with no config file, and nothing is left on disk afterwards.
- Existing CI pipelines that already define `COPR_TOKEN` and `COPR_CONFIG`
  work without duplicating secrets under new names.
- The write paths keep reading the file profile through `Manager.Profile`,
  while `Manager.Effective` composes the environment over the file, so
  ephemeral credentials can never be written into the config by a command that
  was only reading it.
- A command that needs the username and was given no `COPRCTL_USERNAME` pays
  one extra `auth-check` request on the first success, cached per process;
  anonymous reads are unaffected.
- `config show`, `auth status`, and `doctor` name the variables in use, so an
  environment that shadows the file is visible rather than surprising.
