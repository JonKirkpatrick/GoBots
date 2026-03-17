#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "Usage: $0 <name> <owner-token>"
  echo "Example: $0 fhourstones_bot_2 owner_16bfe559c7bc5573dd06e885a2b9b5244459"
  exit 1
fi

BOT_NAME="$1"
OWNER_TOKEN="$2"
SERVER="${BBS_SERVER:-localhost:8080}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$REPO_ROOT"
go run ./cmd/bbs-agent \
  --server "$SERVER" \
  --name "$BOT_NAME" \
  --owner-token "$OWNER_TOKEN" \
  --worker python3 \
  --worker-arg examples/Fhourstones/fhourstones_worker_contract.py
