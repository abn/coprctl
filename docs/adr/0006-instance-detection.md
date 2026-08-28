---
type: Decision
title: ADR 0006 - Instance detection
description: Profiles are named by the instance URL, with well-known deployments recognized.
status: stable
---

## Status

Stable.

## Context

Copr is free software and anyone can host an instance; the well-known public
deployments are Fedora production, staging, and openEuler, but there is no
limit. A user needs a clear way to name and switch between profiles for
different instances, and an import or login flow should pick a sensible default
profile name from the URL rather than forcing the user to invent one.

## Decision

`DetectInstance` names a profile from its base URL: the well-known deployments
(`production` for `copr.fedorainfracloud.org`, `staging` for
`copr.stg.fedoraproject.org`, `openEuler` for `*.openeuler.*`) get friendly
names, and any other URL falls back to its hostname. `config import`,
`auth login`, and `config set` use this to name a profile, defaulting to
production when no URL is given.

## Consequences

- The well-known deployments get stable, friendly profile names.
- Self-hosted instances are first-class: their profile name is the hostname.
- Import and login produce a usable default profile without the user needing
  to choose a name, and the first imported profile becomes the default.