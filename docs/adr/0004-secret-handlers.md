---
type: Decision
title: ADR 0004 - Secret-handler support
description: Tokens can be stored in a system secret handler instead of the config file.
status: stable
---

## Status

Stable.

## Context

API tokens are credentials and should not live in a plaintext config file when
a system secret handler is available. The config already supported a
`token_command` for fetching a token from an external command, but there was no
first-class way to store a token in a secret handler, and `config set token`
could leak the value through the process argv if it was passed as an argument.

## Decision

Add a secret-handler abstraction supporting `pass`, `gopass`, and
`secret-tool`. A profile can name a handler and a key; when set, `Auth()`
resolves the token through the handler before falling back to `token_command`
and then an inline token. `config set token --secret-handler <name>` stores the
value in the handler (read from a prompt or stdin, never argv) and keeps only
the handler reference in the config. `1Password` (`op`) is not supported
because its item references differ.

## Consequences

- Tokens can be kept out of the config file entirely when a handler is present.
- `config set token` never accepts the value as a positional argument, so it
  cannot leak via the process list or shell history.
- The supported handler set is `pass`, `gopass`, and `secret-tool`; other
  backends require an implementation.