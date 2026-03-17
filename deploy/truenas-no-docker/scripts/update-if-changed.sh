#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$DEPLOY_DIR/../.." && pwd)"
LOCK_FILE="$SCRIPT_DIR/.update.lock"
LOG_FILE="$SCRIPT_DIR/update.log"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "[$(date -Iseconds)] update skipped: another update process is running" >>"$LOG_FILE"
  exit 0
fi

if ! git -C "$REPO_ROOT" diff --quiet || ! git -C "$REPO_ROOT" diff --cached --quiet; then
  echo "[$(date -Iseconds)] update aborted: tracked local changes exist in repo" >>"$LOG_FILE"
  exit 1
fi

echo "[$(date -Iseconds)] update check started" >>"$LOG_FILE"

git -C "$REPO_ROOT" fetch --prune origin main >>"$LOG_FILE" 2>&1

LOCAL_SHA="$(git -C "$REPO_ROOT" rev-parse HEAD)"
REMOTE_SHA="$(git -C "$REPO_ROOT" rev-parse origin/main)"

if [[ "$LOCAL_SHA" == "$REMOTE_SHA" ]]; then
  echo "[$(date -Iseconds)] no changes detected" >>"$LOG_FILE"
  exit 0
fi

echo "[$(date -Iseconds)] updating from $LOCAL_SHA to $REMOTE_SHA" >>"$LOG_FILE"
git -C "$REPO_ROOT" pull --ff-only origin main >>"$LOG_FILE" 2>&1

"$SCRIPT_DIR/build-server.sh" >>"$LOG_FILE" 2>&1
"$SCRIPT_DIR/restart-server.sh" >>"$LOG_FILE" 2>&1

echo "[$(date -Iseconds)] update completed successfully" >>"$LOG_FILE"
