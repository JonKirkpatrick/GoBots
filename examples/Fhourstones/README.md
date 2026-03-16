# Fhourstones Worker Integration

This folder contains an imported Fhourstones solver and a worker adapter that plugs into `bbs-agent`.

## Files

- `Game.c`, `TransGame.c`, `SearchGame.c` - original Fhourstones sources
- `fhourstones_worker_contract.py` - worker that speaks `BBS_AGENT_CONTRACT.md`
- `fhourstones` - solver binary (created by build step)

## Build Solver Binary

From repository root:

```bash
cd examples/Fhourstones
gcc -O2 -std=c99 SearchGame.c -o fhourstones
```

The worker can also attempt this build automatically if `fhourstones` is missing and `gcc` is available.

## Run With bbs-agent

From repository root:

```bash
go run ./cmd/bbs-agent \
  --server localhost:8080 \
  --name fhourstones_bot \
  --owner-token owner_... \
  --worker python3 \
  --worker-arg examples/Fhourstones/fhourstones_worker_contract.py
```

Optional:

- `--worker-arg --solve-timeout`
- `--worker-arg 0.8`
- `--worker-arg --solver`
- `--worker-arg /absolute/path/to/fhourstones`

## Behavior Notes

- The worker only acts on `connect4` games.
- It reconstructs a valid move history from board state and evaluates candidate columns with Fhourstones.
- If solver calls time out, it falls back to a center-first legal move.
- Lower timeout is safer for strict move clocks; higher timeout can improve move quality.
