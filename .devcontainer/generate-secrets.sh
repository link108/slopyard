#!/usr/bin/env bash
# Generates this workspace's local PostgreSQL and Redis credentials once,
# and reuses them on every subsequent run. Runs on the HOST via
# devcontainer.json's initializeCommand, before the container/backing
# services start (Compose secrets must exist as files before `docker
# compose up`).
#
# Workspace identity here is just "this checkout's own directory" - the
# secrets live next to this script (repo-relative, gitignored), so every
# git worktree naturally gets its own, with no separate ID to compute or
# track. This matches how the Dev Container CLI itself scopes the Compose
# project (by workspace folder), so both stay consistent for free.
set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
SECRETS_DIR="$SCRIPT_DIR/secrets"

umask 077
mkdir -p "$SECRETS_DIR"
chmod 700 "$SECRETS_DIR"

generate() {
  local file=$1 bytes=$2
  if [ ! -s "$file" ]; then
    openssl rand -hex "$bytes" > "$file"
    chmod 600 "$file"
    echo "generated $(basename "$file")"
  fi
}

generate "$SECRETS_DIR/postgres_user" 8
generate "$SECRETS_DIR/postgres_password" 24
generate "$SECRETS_DIR/redis_password" 24
