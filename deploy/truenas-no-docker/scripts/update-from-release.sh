#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEPLOY_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$DEPLOY_DIR/.env"
LOG_FILE="$SCRIPT_DIR/update-release.log"
LOCK_FILE="$SCRIPT_DIR/.update-release.lock"
BIN_PATH="$DEPLOY_DIR/bin/bbs-server"
TMP_PATH="$BIN_PATH.download"

exec 9>"$LOCK_FILE"
if ! flock -n 9; then
  echo "[$(date -Iseconds)] release update skipped: another update process is running" >>"$LOG_FILE"
  exit 0
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "[$(date -Iseconds)] release update failed: missing $ENV_FILE" >>"$LOG_FILE"
  exit 1
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "[$(date -Iseconds)] release update failed: curl not found" >>"$LOG_FILE"
  exit 1
fi

# shellcheck disable=SC1090
source "$ENV_FILE"

calc_sha() {
  local file="$1"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$file" | awk '{print $1}'
    return
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$file" | awk '{print $1}'
    return
  fi
  echo ""
}

arch_raw="$(uname -m)"
case "$arch_raw" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    echo "[$(date -Iseconds)] release update failed: unsupported architecture $arch_raw" >>"$LOG_FILE"
    exit 1
    ;;
esac

release_owner="${BBS_RELEASE_OWNER:-JonKirkpatrick}"
release_repo="${BBS_RELEASE_REPO:-bbs}"
release_tag="${BBS_RELEASE_TAG:-latest}"
asset_name="${BBS_RELEASE_ASSET:-bbs-server-linux-$arch}"

if [[ -n "${BBS_BINARY_URL:-}" ]]; then
  binary_url="$BBS_BINARY_URL"
else
  if [[ "$release_tag" == "latest" ]]; then
    binary_url="https://github.com/$release_owner/$release_repo/releases/latest/download/$asset_name"
  else
    binary_url="https://github.com/$release_owner/$release_repo/releases/download/$release_tag/$asset_name"
  fi
fi

echo "[$(date -Iseconds)] release update started arch=$arch url=$binary_url" >>"$LOG_FILE"

mkdir -p "$(dirname "$BIN_PATH")"

if ! curl -fL --connect-timeout 15 --max-time 180 "$binary_url" -o "$TMP_PATH" >>"$LOG_FILE" 2>&1; then
  rm -f "$TMP_PATH"
  echo "[$(date -Iseconds)] release update failed: download failed" >>"$LOG_FILE"
  exit 1
fi

chmod +x "$TMP_PATH"

current_sha=""
if [[ -f "$BIN_PATH" ]]; then
  current_sha="$(calc_sha "$BIN_PATH")"
fi
new_sha="$(calc_sha "$TMP_PATH")"

if [[ -n "$current_sha" && -n "$new_sha" && "$current_sha" == "$new_sha" ]]; then
  rm -f "$TMP_PATH"
  echo "[$(date -Iseconds)] release update no-op: binary unchanged" >>"$LOG_FILE"
  exit 0
fi

mv "$TMP_PATH" "$BIN_PATH"
chmod +x "$BIN_PATH"

"$SCRIPT_DIR/restart-server.sh" >>"$LOG_FILE" 2>&1

echo "[$(date -Iseconds)] release update completed" >>"$LOG_FILE"
