---
type: Guide
title: Credentials from the environment
description: Supply Copr credentials through environment variables for CI and ephemeral shells, and persist them only when you ask.
status: stable
---

# Credentials from the environment

Every command can take its credentials from the environment instead of a
config file. Environment credentials always win over the config file, and they
are never written anywhere: set them, run the command, and clean up.

## Variables

| Variable | Meaning |
| --- | --- |
| `COPRCTL_CONFIG` | a verbatim `[copr-cli]` credential block, the same text the Copr API page offers |
| `COPRCTL_TOKEN` | the API token |
| `COPRCTL_LOGIN` | the API login |
| `COPRCTL_USERNAME` | your account username |
| `COPRCTL_URL` | the instance base URL (default: Fedora production) |

The `COPR_` names (`COPR_CONFIG`, `COPR_TOKEN`, `COPR_LOGIN`,
`COPR_USERNAME`, `COPR_URL`) are honoured as a fallback, because that is what
CI pipelines commonly use. When both forms are set, the `COPRCTL_` name wins.
Per-field variables override fields from a block, so a shared block can be
pointed at another login or instance without editing it.

`COPRCTL_CONFIG` takes a `[copr-cli]` block, the text the Copr API page hands
you, not a coprctl `config.toml`; use `--config` for a file path. And
`COPRCTL_URL` and `COPRCTL_USERNAME` are only read alongside a token, or a block
that carries one. On their own they are ignored, so they can never redirect
saved credentials to another instance.

## The login is not optional

Copr API v3 authenticates with HTTP Basic `login:token` and looks the account
up by the API login. There is no token-only path, so a token always needs a
login. If you set `COPRCTL_TOKEN` without `COPRCTL_LOGIN`, the command fails
immediately rather than sending an anonymous request.

The username is a different value from the login. Leave `COPRCTL_USERNAME`
unset and coprctl resolves it through `auth-check`, caching the result for the
rest of the process. That is what lets a bare project reference mean "my
project":

```console
$ COPRCTL_TOKEN="$TOKEN" COPRCTL_LOGIN="$LOGIN" coprctl project list
```

## CI example

```yaml
- name: Submit the build
  env:
    COPR_TOKEN: ${{ secrets.COPR_TOKEN }}
    COPR_LOGIN: ${{ secrets.COPR_LOGIN }}
  run: coprctl build submit abn/helloworld --source url --url "$SPEC_URL"
```

A full block works too, which is convenient when a pipeline already stores one:

```console
$ COPRCTL_CONFIG="$(cat ~/.config/copr)" coprctl build submit abn/helloworld --wait
```

## Another instance

`COPRCTL_URL` selects the instance. Environment credentials never inherit the
URL from a configured profile, so a token is only ever sent to the instance you
name, or to production when you name none:

```console
$ COPRCTL_URL=https://copr.stg.fedoraproject.org \
  COPRCTL_TOKEN="$STG_TOKEN" COPRCTL_LOGIN="$STG_LOGIN" \
  coprctl auth status
```

## Persisting credentials

`auth login` is the only command that writes credentials. Run it with the
environment set and it imports them verbatim, skipping the browser and the
paste:

```console
$ COPRCTL_CONFIG="$(cat ~/.config/copr)" coprctl auth login
```

The result names the profile (the instance name by default) and reports
`source: env`. Pass `--profile` to choose the name, or `--url` to override the
instance.

## Rotating an environment token

`auth rotate` will not rotate environment credentials on its own. A new token
cannot be written back to a variable, and silently discarding it would lock you
out. Pass `--reveal` to print the replacement instead of storing it:

```console
$ COPRCTL_TOKEN="$TOKEN" COPRCTL_LOGIN="$LOGIN" coprctl auth rotate --yes --reveal
```

## Seeing what is in effect

`config show`, `auth status`, and `doctor` report that credentials came from
the environment and name the variables involved, so an environment that
shadows a saved profile is visible. Secrets stay masked unless you pass
`--reveal`.
