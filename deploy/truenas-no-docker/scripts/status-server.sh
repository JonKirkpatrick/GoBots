#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_FILE="$SCRIPT_DIR/bbs-server.pid"

if [[ ! -f "$PID_FILE" ]]; then
  echo "bbs-server status: stopped"
  exit 1
fi

pid="$(cat "$PID_FILE")"
if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
  echo "bbs-server status: running (pid=$pid)"
  exit 0
fi

echo "bbs-server status: stopped (stale pid file)"
exit 1
