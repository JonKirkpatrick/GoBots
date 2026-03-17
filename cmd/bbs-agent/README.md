# bbs-agent (Minimal v0.2)

`bbs-agent` is a local bridge process that:

- launches a worker process
- connects to the BBS TCP server
- performs `REGISTER`
- forwards only actionable `turn` messages to the worker
- forwards worker `action` messages back as `MOVE`

It implements `BBS_AGENT_CONTRACT.md` v0.2.

## Runtime Contract

- Agent -> Worker: `welcome`, `turn`, `shutdown`
- Worker -> Agent: `action`, `log`

## Quick Run (Python Worker Template)

From repository root:

```bash
go run ./cmd/bbs-agent \
  --server localhost:8080 \
  --name agent_python_bot \
  --owner-token owner_... \
  --worker python3 \
  --worker-arg examples/python_worker_contract_template.py
```

Optional:

- `--credentials-file path/to/creds.txt`
- `--capabilities connect4`
- `--worker-arg ...` (repeatable)

## Notes

- The agent writes credentials to `<name>_credentials.txt` if the server issues a new identity during register.
- Dashboard/admin flows remain server-side; worker protocol is focused on turn-time decisioning.
- Current BBS transport is plain TCP; treat credentials and owner tokens as sensitive.
