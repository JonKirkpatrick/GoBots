# BBS Agent Contract v0.2 (Minimal Turn Loop)

This contract defines a minimal stdin/stdout JSONL protocol between:

- `bbs-agent` (network bridge to BBS server)
- `worker` (bot logic process)

Goal: keep the runtime message surface small and deterministic.

## Design

- Worker does not perform networking.
- Dashboard/admin flows stay outside the worker protocol.
- After one `welcome`, runtime loop is only `turn` -> `action`.

## Transport

- Encoding: UTF-8
- Framing: one JSON object per line
- Envelope:

```json
{
  "v": "0.2",
  "type": "message_type",
  "id": "optional",
  "payload": {}
}
```

## Agent -> Worker

### `welcome`

Sent once after successful `JOIN`.

```json
{
  "v": "0.2",
  "type": "welcome",
  "payload": {
    "agent_name": "bbs-agent",
    "agent_version": "0.2.0",
    "server": "localhost:8080",
    "session_id": 12,
    "arena_id": 3,
    "player_id": 1,
    "env": "connect4",
    "time_limit_ms": 1000,
    "effective_time_limit_ms": 1200,
    "capabilities": ["connect4"]
  }
}
```

### `turn`

Sent when an actionable state is available for the worker.

```json
{
  "v": "0.2",
  "type": "turn",
  "payload": {
    "step": 7,
    "deadline_ms": 1200,
    "obs": {
      "raw_state": "{\"board\":[...],\"turn\":1}",
      "state_obj": {
        "board": [[0,0,0,0,0,0,0], [0,0,0,0,0,0,0]],
        "turn": 1
      },
      "turn_player": 1,
      "your_turn": true,
      "source": "server_data"
    },
    "response": {
      "type": "move",
      "status": "ok",
      "payload": "accepted"
    },
    "reward": 0.0,
    "done": false,
    "truncated": false
  }
}
```

Notes:

- `obs` is game/environment state.
- `response` carries result of a prior action when available.
- `done` and `truncated` indicate terminal conditions.

### `shutdown`

Agent is terminating.

```json
{
  "v": "0.2",
  "type": "shutdown",
  "payload": {
    "reason": "operator_exit"
  }
}
```

## Worker -> Agent

### `action`

Worker submits the next action.

```json
{
  "v": "0.2",
  "type": "action",
  "payload": {
    "action": "3"
  }
}
```

### `log` (optional)

Worker log line forwarded to agent stderr.

```json
{
  "v": "0.2",
  "type": "log",
  "payload": {
    "level": "info",
    "message": "picked action=3"
  }
}
```

## Migration Notes

v0.2 replaces v0.1 runtime chatter (`hello`, `hello_ack`, `registered`, `manifest`, `state`, `event`, `move`) with a smaller interaction loop.
