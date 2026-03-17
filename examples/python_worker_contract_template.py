#!/usr/bin/env python3
"""Python worker template for BBS Agent Contract v0.2.

Protocol shape:
- agent -> worker: `welcome`, `turn`, `shutdown`
- worker -> agent: `action`, `log`

The turn payload follows an RL-style loop:
- obs, response, reward, done, truncated, deadline_ms
"""

from __future__ import annotations

import json
import random
import sys
from typing import Any, Dict, Optional

CONTRACT_VERSION = "0.2"


def send_message(msg_type: str, payload: Dict[str, Any], msg_id: Optional[str] = None) -> None:
    msg: Dict[str, Any] = {
        "v": CONTRACT_VERSION,
        "type": msg_type,
        "payload": payload,
    }
    if msg_id:
        msg["id"] = msg_id

    sys.stdout.write(json.dumps(msg, ensure_ascii=True) + "\n")
    sys.stdout.flush()


def log(level: str, message: str) -> None:
    send_message("log", {"level": level, "message": message})


class WorkerState:
    def __init__(self) -> None:
        self.session_id: Optional[int] = None
        self.arena_id: Optional[int] = None
        self.player_id: Optional[int] = None
        self.env: str = ""
        self.effective_time_limit_ms: int = 0


def normalized_name(name: str) -> str:
    return name.strip().lower()


def is_empty_cell(cell: Any) -> bool:
    if isinstance(cell, bool):
        return False
    if isinstance(cell, (int, float)):
        return int(cell) == 0
    if isinstance(cell, str):
        return cell.strip() in {"", "0"}
    return False


def as_int(value: Any, default: int = 0) -> int:
    if isinstance(value, bool):
        return default
    if isinstance(value, (int, float)):
        return int(value)
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return default
        try:
            return int(text)
        except ValueError:
            return default
    return default


def connect4_legal_columns(obs: Dict[str, Any]) -> list[str]:
    state_obj = obs.get("state_obj")
    if not isinstance(state_obj, dict):
        return []

    board = state_obj.get("board")
    if not isinstance(board, list) or not board:
        return []

    top_row = board[0]
    if not isinstance(top_row, list) or not top_row:
        return []

    legal: list[str] = []
    for col, cell in enumerate(top_row):
        if is_empty_cell(cell):
            legal.append(str(col))
    return legal


def is_our_turn(obs: Dict[str, Any], worker_state: WorkerState) -> bool:
    if "your_turn" in obs:
        return bool(obs.get("your_turn"))
    turn_player = as_int(obs.get("turn_player"), default=0)
    return worker_state.player_id is not None and turn_player == worker_state.player_id


def choose_action(obs: Dict[str, Any], worker_state: WorkerState) -> Optional[str]:
    if normalized_name(worker_state.env) == "connect4":
        legal = connect4_legal_columns(obs)
        if legal:
            return random.choice(legal)

    legal_moves = obs.get("legal_moves")
    if isinstance(legal_moves, list) and legal_moves:
        return str(random.choice(legal_moves))

    return None


def handle_welcome(payload: Dict[str, Any], worker_state: WorkerState) -> None:
    worker_state.session_id = as_int(payload.get("session_id"), default=0)
    worker_state.arena_id = as_int(payload.get("arena_id"), default=0)
    worker_state.player_id = as_int(payload.get("player_id"), default=0)
    worker_state.env = str(payload.get("env") or "")
    worker_state.effective_time_limit_ms = as_int(payload.get("effective_time_limit_ms"), default=0)
    if worker_state.effective_time_limit_ms <= 0:
        worker_state.effective_time_limit_ms = as_int(payload.get("time_limit_ms"), default=0)

    log(
        "info",
        (
            f"welcome session_id={worker_state.session_id} arena_id={worker_state.arena_id} "
            f"player_id={worker_state.player_id} env={worker_state.env} "
            f"move_limit_ms={worker_state.effective_time_limit_ms}"
        ),
    )


def handle_turn(payload: Dict[str, Any], worker_state: WorkerState) -> None:
    if bool(payload.get("done")):
        return

    obs = payload.get("obs")
    if not isinstance(obs, dict):
        return

    if not is_our_turn(obs, worker_state):
        return

    action = choose_action(obs, worker_state)
    if action is None:
        return

    send_message("action", {"action": action})


def main() -> int:
    worker_state = WorkerState()

    for raw_line in sys.stdin:
        line = raw_line.strip()
        if not line:
            continue

        try:
            msg = json.loads(line)
        except json.JSONDecodeError:
            log("error", f"invalid JSON from agent: {line[:120]}")
            continue

        if not isinstance(msg, dict):
            log("error", "invalid envelope: expected object")
            continue

        version = str(msg.get("v", ""))
        msg_type = str(msg.get("type", ""))
        payload = msg.get("payload")

        if version != CONTRACT_VERSION:
            log("error", f"unsupported contract version: {version}")
            continue

        if not isinstance(payload, dict):
            payload = {}

        if msg_type == "welcome":
            handle_welcome(payload, worker_state)
            continue

        if msg_type == "turn":
            handle_turn(payload, worker_state)
            continue

        if msg_type == "shutdown":
            log("info", "shutdown requested")
            return 0

        log("debug", f"ignored message type={msg_type}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
