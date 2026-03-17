# TrueNAS SCALE Deployment (No Docker)

This setup runs `bbs-server` directly as a background process on your TrueNAS host.

Use this if you do not want to use Docker/Apps.

## What You Get

- Always-on `bbs-server` process
- Dashboard on port `3000`
- Bot endpoint on port `8080`
- Optional nightly update/rebuild/restart from GitHub

## Important Requirements

- You need `git` on the host shell.
- Server runtime state is in-memory. Restarts clear active runtime state/history.

For updates/builds, you have two options:

- `go` available on TrueNAS: build from source with `scripts/build-server.sh`
- no `go` on TrueNAS: download prebuilt release binary with `scripts/update-from-release.sh`

## 1. Clone To A Persistent Dataset

Example path (adjust to your pool):

```bash
mkdir -p /mnt/<pool>/apps
cd /mnt/<pool>/apps
git clone https://github.com/JonKirkpatrick/bbs.git
cd bbs/deploy/truenas-no-docker
```

Use the same base path in any cron command examples below.

## 2. Configure Environment

```bash
cp .env.example .env
```

Edit `.env` and set:

```bash
BBS_DASHBOARD_ADMIN_KEY=<long-random-secret>
```

Optional:

- `TZ=UTC`
- `BBS_BINARY_URL` (direct binary URL)
- or release fields (`BBS_RELEASE_OWNER`, `BBS_RELEASE_REPO`, `BBS_RELEASE_TAG`, `BBS_RELEASE_ASSET`)

## 3. Make Scripts Executable

```bash
chmod +x scripts/*.sh
```

## 4. Build And Start

```bash
./scripts/build-server.sh
./scripts/start-server.sh
./scripts/status-server.sh
```

## 4b. No-Go Host: Download Binary And Start

If your TrueNAS host does not have Go installed:

```bash
./scripts/update-from-release.sh
./scripts/status-server.sh
```

The release updater expects GitHub release assets named like:

- `bbs-server-linux-amd64`
- `bbs-server-linux-arm64`

If you host your own fork, publish these assets in Releases first.
This repo includes `.github/workflows/release-bbs-server.yml`, which publishes both assets when you push a `v*` tag.
Tag names are case-sensitive; use lowercase `v` (example: `v0.0.2`).

Dashboard URL:

- `http://<truenas-ip>:3000`

Bot endpoint:

- `<truenas-ip>:8080`

## 5. Daily Operations

```bash
./scripts/status-server.sh
./scripts/restart-server.sh
./scripts/stop-server.sh
```

Logs:

- runtime log: `scripts/bbs-server.log`
- build log: `scripts/build.log`
- update log: `scripts/update.log`
- release update log: `scripts/update-release.log`

## 6. Optional Cron Jobs

Create TrueNAS cron jobs in `System Settings` -> `Advanced` -> `Cron Jobs`.

1. Nightly source update/rebuild/restart at 00:00 (requires Go):

```bash
/usr/bin/env bash /mnt/<pool>/apps/bbs/deploy/truenas-no-docker/scripts/update-if-changed.sh
```

Alternative for hosts without Go (download prebuilt release binary):

```bash
/usr/bin/env bash /mnt/<pool>/apps/bbs/deploy/truenas-no-docker/scripts/update-from-release.sh
```

2. Optional forced restart at 00:10:

```bash
/usr/bin/env bash /mnt/<pool>/apps/bbs/deploy/truenas-no-docker/scripts/restart-server.sh
```

If you prefer fewer interruptions, keep only one updater job.

## Startup On Boot (Simple Cron @reboot)

Add a cron job that runs on reboot:

```bash
/usr/bin/env bash /mnt/<pool>/apps/bbs/deploy/truenas-no-docker/scripts/start-server.sh
```

## Troubleshooting

- `go: command not found`: use `scripts/update-from-release.sh` or copy a prebuilt binary into `bin/bbs-server`.
- `missing .env`: run `cp .env.example .env` and set admin key.
- `curl: (23) Failure writing output to destination`: update scripts (`git pull`) and ensure `deploy/truenas-no-docker/bin/` exists.
- process exits immediately: check `scripts/bbs-server.log`.
