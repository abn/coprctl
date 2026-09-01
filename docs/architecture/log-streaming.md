---
type: Reference
title: Log streaming
description: How coprctl tails live multi-chroot build logs.
status: stable
---

# Log streaming

`coprctl log tail` streams build logs from every chroot of a build
concurrently. It is the highest-value missing capability the project set out
to fix, and the piece with the most real engineering in it.

## Target resolution

`log tail` accepts any of:

- `BUILD_ID` - every chroot of that build concurrently.
- `BUILD_ID/CHROOT` - one chroot.
- `REF/PKG` - the latest build of that package.

Each target resolves to a build chroot, whose authoritative `result_url` points
at the log files. The source-build phase lives under `srpm-builds/<id>`.

`coprctl monitor OWNER/PROJECT` exposes the same source files: each chroot's
JSON output carries `url_build_log` and `url_backend_log`. The monitor URL and
the tailer's `Locate` resolve the same `builder-live.log`, but not the same
bytes. `Locate` always appends `.gz` to the chroot `result_url`; the monitor
URL points at the uncompressed live log while a chroot runs, switches to the
`.gz` form once the chroot is terminal, and is `.gz` throughout `importing`.
Pending, waiting, and starting chroots emit no URL at all.

## The live log problem

`builder-live.log.gz` is a gzip file being appended to while the build runs.
Whether the tailer can resume with an HTTP `Range` request depends on how the
writer flushes gzip members:

- If each flush closes a gzip member (multistream gzip), a byte range starting
  at a member boundary is itself a valid gzip stream, so incremental resume is
  cheap.
- If it flushes inside one long member, a mid-stream range is undecodable, so
  incremental resume is impossible.

The tailer never assumes which framing the backend uses. It probes and falls
back.

## Fetch strategies

**Strategy A - incremental (preferred).** Issue a `Range: bytes=<offset>-`
request with `Accept-Encoding: identity` (so the HTTP client does not
transparently gunzip and break byte accounting). A `206` whose body starts with
the gzip magic `1f 8b` is a valid member boundary, so decode and advance the
offset. A `416` means no new bytes. If the response is not a valid member
boundary or the server ignores `Range` (a `200`), the stream is marked
non-resumable and falls back to Strategy B permanently.

**Strategy B - full refetch (always correct).** Re-fetch the whole log and skip
the already-emitted decompressed bytes. Cost is `O(log size)` per poll, so the
tailer backs off adaptively as the log grows.

## Correctness rules

- **No torn lines.** A partial trailing line is buffered and only emitted once
  it ends in a newline, so an agent parsing JSONL never sees a half line.
- **No duplicates.** Emitted decompressed bytes are tracked so a full refetch
  does not repeat already-seen content.
- **Terminal detection.** The tailer stops when the owning build chroot reaches
  a terminal state and one final fetch returns no new bytes, not on state
  alone (or it would truncate the last lines).
- **Backpressure.** A bounded per-stream channel drops from the middle and
  emits an explicit `log.truncated` marker rather than silently losing lines.
- **Concurrency.** A bounded worker pool covers every target with a configurable
  maximum number of simultaneous streams.

## Emission

Log lines and truncation markers are published onto the shared event bus. The
plain prefixed writer, the JSONL writer, and the TUI are all consumers of that
same bus, so there is exactly one implementation of "what is happening". See
[the agent contract](agent-contract.md) for the event schema.