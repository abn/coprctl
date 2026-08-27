#!/usr/bin/env bash
# Idempotent bootstrap for tool shims and local-only agent state.
# Committed; safe to re-run at any time.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHIMS="$REPO_ROOT/.agents/shims"
mkdir -p "$SHIMS"

# Claude Code reads AGENTS.md; the shim keeps Claude-specific state out of the
# committed tree. Symlinked so the shim stays fresh if AGENTS.md moves.
if command -v claude >/dev/null 2>&1 || [ -n "${CLAUDE_CODE_ENTRYPOINT:-}" ]; then
  if [ ! -f "$REPO_ROOT/CLAUDE.md" ]; then
    printf 'Read the project contract in AGENTS.md.\n' > "$REPO_ROOT/CLAUDE.md"
  fi
fi

echo "bootstrap: ready at $REPO_ROOT"