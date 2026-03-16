# Dashboard Remote Quickstart

This guide is for users who want to connect a bot to an existing Build-a-Bot Stadium server.

It assumes you can open the dashboard URL in a browser, but you are not running the server locally.

## What You Need

- Dashboard URL (for example: `https://bbs.example.com`)
- Bot TCP endpoint (for example: `bbs.example.com:8080`)
- Python 3.9+ if you want to use the included template bot

## Fast Path (Dashboard + Bot)

1. Open the dashboard in your browser.
2. Click `Register Bot`.
3. In `Bot Control`, copy the token using `Copy token`.
4. Note the `Bot Host/IP` and `Bot Port` shown in the same panel.
5. Start your bot and include the owner token in `REGISTER`.
6. Return to the dashboard and wait for `Linked to session #...`.
7. Use owner controls to create an arena, join an arena, or disconnect your bot.

## Python Template Bot

A ready template is included at `examples/python_bot_template.py`.

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

The bot loads `bot_id` and `bot_secret` from that file and reuses them.

### Optional Flags

- `--capabilities connect4,chess` to advertise capability tags during `REGISTER`
- `--credentials-file <path>` to control where credentials are read/written

## What The Template Handles

- Connects to the server endpoint
- Sends `REGISTER` automatically
- Parses JSON responses from the server
- Prints readable summaries for arena-related response types (`create`, `join`, `list`, `data`, `move`, `gameover`, `leave`, `timeout`, `ejected`)
- Keeps reading server output and allows you to type commands interactively

## Common Commands After Register

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
