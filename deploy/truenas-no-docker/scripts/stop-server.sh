#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_FILE="$SCRIPT_DIR/bbs-server.pid"

if [[ ! -f "$PID_FILE" ]]; then
  echo "bbs-server is not running (no pid file)"
  exit 0
fi

pid="$(cat "$PID_FILE")"
if [[ -z "$pid" ]]; then
  rm -f "$PID_FILE"
  echo "bbs-server is not running (empty pid file)"
  exit 0
fi

if ! kill -0 "$pid" >/dev/null 2>&1; then
  rm -f "$PID_FILE"
  echo "bbs-server is not running (stale pid file removed)"
  exit 0
fi

kill "$pid"
for _ in {1..20}; do
  if ! kill -0 "$pid" >/dev/null 2>&1; then
    rm -f "$PID_FILE"
    echo "bbs-server stopped"
    exit 0
  fi
  sleep 0.25
done

kill -9 "$pid"
rm -f "$PID_FILE"
echo "bbs-server force-stopped"
