#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$DEPLOY_DIR/../.." && pwd)"
ENV_FILE="$DEPLOY_DIR/.env"
PID_FILE="$SCRIPT_DIR/bbs-server.pid"
LOG_FILE="$SCRIPT_DIR/bbs-server.log"
BIN_PATH="$DEPLOY_DIR/bin/bbs-server"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "missing $ENV_FILE (copy from .env.example first)" >&2
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

if [[ -z "${BBS_DASHBOARD_ADMIN_KEY:-}" ]]; then
  echo "BBS_DASHBOARD_ADMIN_KEY is required in $ENV_FILE" >&2
  exit 1
fi

if [[ ! -x "$BIN_PATH" ]]; then
  echo "binary missing; building first..."
  "$SCRIPT_DIR/build-server.sh"
fi

if [[ -f "$PID_FILE" ]]; then
  pid="$(cat "$PID_FILE")"
  if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
    echo "bbs-server already running (pid=$pid)"
    exit 0
  fi
  rm -f "$PID_FILE"
fi

(
  cd "$REPO_ROOT/cmd/bbs-server"
  export BBS_DASHBOARD_ADMIN_KEY
  export TZ="${TZ:-UTC}"
  nohup "$BIN_PATH" >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
)

sleep 1
pid="$(cat "$PID_FILE")"
if kill -0 "$pid" >/dev/null 2>&1; then
  echo "bbs-server started (pid=$pid)"
  echo "dashboard: http://$(hostname -I 2>/dev/null | awk '{print $1}' || echo '<host-ip>'):3000"
  exit 0
fi

echo "failed to start bbs-server; check $LOG_FILE" >&2
exit 1
