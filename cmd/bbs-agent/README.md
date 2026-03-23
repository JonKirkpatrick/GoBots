# bbs-agent (Local Bridge)

`bbs-agent` exposes local JSONL sockets for bot logic and control, and can optionally connect to the BBS TCP server.

Primary mode on linux/mac:

- agent listens on bot Unix socket (`--listen`)
- agent listens on control Unix socket (`--control-listen`, or default derived path)
- local bot connects and sends `hello`
- when `--server` is provided: bot receives `welcome`/`turn` and returns `action`
- when `--server` is omitted: agent runs in local-only mode

Protocol reference: `../../docs/reference/BBS_AGENT_CONTRACT.md`

## Why Use It

- isolates bot code from raw BBS TCP protocol details
- supports any contract-compliant arena interaction pattern
- central place for credentials/session handling

## Quick Run

Terminal 1 (agent):

```bash
go run ./cmd/bbs-agent \
  --listen /tmp/bbs-agent.sock
```

Optional server-backed mode:

```bash
go run ./cmd/bbs-agent \
  --server localhost:8080 \
  --listen /tmp/bbs-agent.sock
```

Terminal 2 (Python template bot):

```bash
python3 examples/python_socket_bot_template.py \
  --socket /tmp/bbs-agent.sock \
  --name agent_python_bot \
  --owner-token owner_...
```

## Flags

- `--server host:port` optional BBS endpoint
- `--listen` local endpoint (`unix:///tmp/bbs-agent.sock` or `/tmp/bbs-agent.sock`)
- `--control-listen` control endpoint (`unix:///tmp/bbs-agent.sock.control` by default)
- `--register-timeout` registration response timeout

Registration fields (`name`, `owner_token`, capabilities, credentials) come from bot `hello` payload when `--server` is used.

Initial control message types for `--control-listen` are:

- `ping` -> `pong`
- `status` -> `status`
- `server_access` -> `server_access`
- `arm` -> `arm_ack`
- `disarm` -> `disarm_ack`
- `lifecycle` -> `lifecycle`
- `quit` -> `quit_ack`

The control socket is for client-to-agent orchestration only and does not proxy raw server commands.
When control requests include `id`, responses echo the same `id`.
