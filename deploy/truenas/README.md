# TrueNAS SCALE Deployment (Always-On `bbs-server`)

This guide sets up an always-on `bbs-server` on your TrueNAS SCALE box so you can:

- bookmark one stable dashboard URL
- point all bots at one stable LAN endpoint
- optionally auto-update nightly from GitHub

## What This Deploys

- Bot TCP endpoint: `:8080`
- Dashboard HTTP endpoint: `:3000`
- Container restart policy: `unless-stopped`

## Prerequisites

- TrueNAS SCALE shell access (SSH or web shell)
- Docker engine + compose plugin available on host
- A persistent dataset path for this repo clone

## 1. Clone To A Persistent Dataset

Example (adjust to your pool/dataset):

```bash
mkdir -p /mnt/<pool>/apps
cd /mnt/<pool>/apps
git clone https://github.com/JonKirkpatrick/bbs.git
cd bbs/deploy/truenas
```

Use the same base path in any cron command examples below.

## 2. Create Runtime Env File

```bash
cp .env.example .env
```

Edit `.env` and set a strong admin key:

```bash
BBS_DASHBOARD_ADMIN_KEY=<long-random-secret>
```

Optional:

- `BBS_BOT_PORT` host-side port mapping for bot connections (default `8080`)
- `BBS_DASHBOARD_PORT` host-side port mapping for dashboard (default `3000`)
- `TZ` for container timezone

## 3. Build And Start

From `deploy/truenas`:

```bash
docker compose --env-file .env up -d --build
```

## 4. Verify

```bash
docker compose ps
docker compose logs --tail=200 bbs-server
```

Open dashboard in browser:

- `http://<truenas-ip>:3000`

Bot endpoint for agents/bots:

- `<truenas-ip>:8080`

## 5. Optional Nightly Automation

Scripts included:

- `scripts/update-if-changed.sh`: fetches `origin/main`, rebuilds and restarts only if changed
- `scripts/restart-server.sh`: force restarts container

### Suggested Cron Jobs

Create two cron tasks in TrueNAS UI (`System Settings` -> `Advanced` -> `Cron Jobs`):

1. Nightly update check (00:00):

```bash
/usr/bin/env bash /mnt/<pool>/apps/bbs/deploy/truenas/scripts/update-if-changed.sh
```

2. Optional nightly restart (00:10):

```bash
/usr/bin/env bash /mnt/<pool>/apps/bbs/deploy/truenas/scripts/restart-server.sh
```

If you prefer minimal disruption, skip the forced restart and keep only `update-if-changed.sh`.

## Operational Notes

- Current server state is in-memory. Any restart clears active runtime state/history.
- Keep this host clone clean (no tracked file edits) so update script can fast-forward safely.
- Logs are written to:
  - `deploy/truenas/scripts/update.log`
  - `deploy/truenas/scripts/restart.log`

## Manual Operations

From `deploy/truenas`:

```bash
# stop
docker compose --env-file .env down

# start
docker compose --env-file .env up -d

# rebuild after local changes
docker compose --env-file .env up -d --build
```
