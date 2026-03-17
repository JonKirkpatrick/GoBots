#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_FILE="$SCRIPT_DIR/restart.log"

if ! command -v docker >/dev/null 2>&1; then
  echo "[$(date -Iseconds)] restart failed: docker is not installed" >>"$LOG_FILE"
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "[$(date -Iseconds)] restart failed: docker compose plugin not available" >>"$LOG_FILE"
  exit 1
fi

cd "$DEPLOY_DIR"
docker compose --env-file .env restart bbs-server >>"$LOG_FILE" 2>&1

echo "[$(date -Iseconds)] restart completed" >>"$LOG_FILE"
