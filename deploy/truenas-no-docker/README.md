# TrueNAS SCALE Deployment (No Docker)

This setup runs `bbs-server` directly as a background process on your TrueNAS host.

Use this if you do not want to use Docker/Apps.

## What You Get

- Always-on `bbs-server` process
- Dashboard on port `3000`
- Bot endpoint on port `8080`
- Optional nightly update/rebuild/restart from GitHub

## Important Requirements

- You need `git` and `go` installed on the host shell.
- Server runtime state is in-memory. Restarts clear active runtime state/history.

If `go` is missing, either:

- install Go on the host, or
- build the binary elsewhere and copy it to `deploy/truenas-no-docker/bin/bbs-server`.

## 1. Clone To A Persistent Dataset

Example path (adjust to your pool):

```bash
mkdir -p /mnt/tank/apps
cd /mnt/tank/apps
git clone https://github.com/JonKirkpatrick/bbs.git
cd bbs/deploy/truenas-no-docker
```

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

## 6. Optional Cron Jobs

Create TrueNAS cron jobs in `System Settings` -> `Advanced` -> `Cron Jobs`.

1. Nightly update/rebuild/restart at 00:00:

```bash
/usr/bin/env bash /mnt/tank/apps/bbs/deploy/truenas-no-docker/scripts/update-if-changed.sh
```

2. Optional forced restart at 00:10:

```bash
/usr/bin/env bash /mnt/tank/apps/bbs/deploy/truenas-no-docker/scripts/restart-server.sh
```

If you prefer fewer interruptions, keep only `update-if-changed.sh`.

## Startup On Boot (Simple Cron @reboot)

Add a cron job that runs on reboot:

```bash
/usr/bin/env bash /mnt/tank/apps/bbs/deploy/truenas-no-docker/scripts/start-server.sh
```

## Troubleshooting

- `go: command not found`: install Go or copy a prebuilt binary into `bin/bbs-server`.
- `missing .env`: run `cp .env.example .env` and set admin key.
- process exits immediately: check `scripts/bbs-server.log`.
