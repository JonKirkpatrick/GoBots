# Dashboard Remote Quickstart

This guide is for users who want to connect a bot to an existing Build-a-Bot Stadium server.

It assumes you can open the dashboard URL in a browser, but you are not running the server locally.

## What You Need

- Dashboard URL (for example: `https://bbs.example.com`)
- Bot TCP endpoint (for example: `bbs.example.com:8080`)
- Python 3.9+ for the included bot templates

## Fast Path (Dashboard + bbs-agent)

The recommended approach is `bbs-agent` — a Go sidecar that handles all BBS TCP networking and forwards game state to a Python worker over stdin/stdout. Your worker only needs to implement decision logic.

1. Open the dashboard in your browser.
2. Click `Register Bot`.
3. In `Bot Control`, copy the token using `Copy token`.
4. Note the `Bot Host/IP` and `Bot Port` shown in the same panel.
5. From the repository root, run `bbs-agent` with your worker:

```bash
go run ./cmd/bbs-agent \
  --server bbs.example.com:8080 \
  --name my_bot \
  --owner-token owner_abc123... \
  --worker python3 \
  --worker-arg examples/python_worker_contract_template.py
```

6. Return to the dashboard and wait for `Linked to session #...`.
7. Use the owner controls to create an arena, join an arena, leave an arena, or disconnect your bot.

See `BBS_AGENT_CONTRACT.md` for the full stdin/stdout JSONL protocol your worker must speak.
See `examples/python_worker_contract_template.py` for a ready-to-run Python worker.
See `cmd/bbs-agent/README.md` for all `bbs-agent` flags.

### Fhourstones Solver Worker

If you want to use the Fhourstones perfect Connect4 solver instead:

```bash
go run ./cmd/bbs-agent \
  --server bbs.example.com:8080 \
  --name fhourstones_bot \
  --owner-token owner_abc123... \
  --worker python3 \
  --worker-arg examples/Fhourstones/fhourstones_worker_contract.py
```

Build the solver binary once with `gcc -O2 -std=c99 examples/Fhourstones/SearchGame.c -o examples/Fhourstones/fhourstones` (or let the worker auto-build it on first run). See `examples/Fhourstones/README.md` for tuning notes.

---

## Alternative: Direct TCP Template (Legacy)

If you prefer to handle the wire protocol yourself, a direct TCP template is at `examples/python_bot_template.py`. This requires no extra processes but gives you raw BBS JSON directly rather than the enriched state the agent provides.

### First Run (new identity)

```bash
python3 examples/python_bot_template.py \
  --server bbs.example.com:8080 \
  --name my_bot \
  --owner-token owner_abc123...
```

If no credentials file is provided, the bot writes a credentials file in the current directory after a successful new registration:

- `<bot_name>_credentials.txt`

### Returning Run (reuse identity)

```bash
python3 examples/python_bot_template.py \
  --server bbs.example.com:8080 \
  --name my_bot \
  --credentials-file my_bot_credentials.txt \
  --owner-token owner_abc123...
```

### Optional Flags

- `--capabilities connect4,chess` to advertise capability tags during `REGISTER`
- `--credentials-file <path>` to control where credentials are read/written

---

## Dashboard Owner Controls

Once your bot is linked, the dashboard Bot Control panel shows:

| Action | Effect |
|---|---|
| Create Arena | Create a new waiting arena |
| Join Arena | Enter an existing arena as a player |
| Leave Arena | Exit the current arena; bot stays connected and can rejoin |
| Disconnect Bot | Close the TCP connection entirely |

## Common TCP Commands After Register

```text
LIST
CREATE connect4 1000 false
JOIN 1 0
MOVE 3
LEAVE
QUIT
```

## Troubleshooting

- `Must REGISTER first`: Registration failed or was not sent.
- `owner token is invalid`: Re-copy from dashboard and retry.
- `owner token is already linked to another active session`: Disconnect the existing linked bot first.
- `arena full`: The target arena already has two players.
- `Clipboard copy failed`: Copy manually from the token field.

## Security Notes

- `owner_token`, `bot_id`, and `bot_secret` are sensitive.
- Treat the credentials file like a secret.
- Current transport is plain TCP; avoid exposing secrets on untrusted networks.
