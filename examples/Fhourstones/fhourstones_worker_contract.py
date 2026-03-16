#!/usr/bin/env python3
"""Fhourstones-powered worker for BBS Agent Contract v0.1.

This worker:
- consumes contract messages from stdin
- derives a legal move sequence from Connect4 board state
- evaluates candidate moves with the Fhourstones solver binary
- emits `move` back to bbs-agent

If solver evaluation is unavailable or times out, it falls back to a legal move policy.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from functools import lru_cache
from pathlib import Path
from typing import Any, Dict, List, Optional, Sequence, Tuple

CONTRACT_VERSION = "0.1"
LOSS = 1
DRAWLOSS = 2
DRAW = 3
DRAWWIN = 4
WIN = 5


class WorkerState:
    def __init__(self, solver_path: Path, solve_timeout_seconds: float) -> None:
        self.session_id: Optional[int] = None
        self.arena_id: Optional[int] = None
        self.player_id: Optional[int] = None
        self.game: str = ""
        self.solver_path = solver_path
        self.solve_timeout_seconds = solve_timeout_seconds


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


def normalized_game_name(name: str) -> str:
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


def extract_board_and_turn(state_payload: Dict[str, Any]) -> Tuple[Optional[List[List[int]]], Optional[int]]:
    state_obj = state_payload.get("state_obj")
    if not isinstance(state_obj, dict):
        return None, None

    board_raw = state_obj.get("board")
    if not isinstance(board_raw, list) or not board_raw:
        return None, None

    board: List[List[int]] = []
    width: Optional[int] = None
    for row in board_raw:
        if not isinstance(row, list):
            return None, None
        parsed_row = [as_int(cell, default=-1) for cell in row]
        if any(cell not in {0, 1, 2} for cell in parsed_row):
            return None, None
        if width is None:
            width = len(parsed_row)
            if width == 0:
                return None, None
        elif len(parsed_row) != width:
            return None, None
        board.append(parsed_row)

    turn = as_int(state_payload.get("turn_player"), default=0)
    if turn not in {1, 2}:
        turn = as_int(state_obj.get("turn"), default=0)
    if turn not in {1, 2}:
        turn = None

    return board, turn


def legal_columns(board: Sequence[Sequence[int]]) -> List[int]:
    if not board:
        return []
    top = board[0]
    return [col for col, cell in enumerate(top) if cell == 0]


def apply_move(board: Sequence[Sequence[int]], col: int, player: int) -> Optional[List[List[int]]]:
    rows = len(board)
    cols = len(board[0]) if rows else 0
    if col < 0 or col >= cols:
        return None

    out = [list(row) for row in board]
    for row in range(rows - 1, -1, -1):
        if out[row][col] == 0:
            out[row][col] = player
            return out
    return None


def board_to_tuple(board: Sequence[Sequence[int]]) -> Tuple[Tuple[int, ...], ...]:
    return tuple(tuple(int(c) for c in row) for row in board)


def top_piece_row(board: Sequence[Sequence[int]], col: int) -> Optional[int]:
    for row in range(len(board)):
        if board[row][col] != 0:
            return row
    return None


def reconstruct_move_sequence(board: Sequence[Sequence[int]], turn: int) -> Optional[List[int]]:
    cols = len(board[0])
    board_t = board_to_tuple(board)
    pieces = sum(1 for row in board for cell in row if cell != 0)
    last_player = 1 if turn == 2 else 2

    @lru_cache(maxsize=None)
    def dfs(state: Tuple[Tuple[int, ...], ...], player_to_remove: int, remaining: int) -> Optional[Tuple[int, ...]]:
        if remaining == 0:
            return ()

        working = [list(row) for row in state]
        candidate_cols: List[int] = []
        for col in range(cols):
            r = top_piece_row(working, col)
            if r is not None and working[r][col] == player_to_remove:
                candidate_cols.append(col)

        # Center-first for tie-break stability.
        center = cols // 2
        candidate_cols.sort(key=lambda c: abs(c - center))

        next_player = 1 if player_to_remove == 2 else 2
        for col in candidate_cols:
            next_state = [row[:] for row in working]
            r = top_piece_row(next_state, col)
            if r is None:
                continue
            next_state[r][col] = 0
            suffix = dfs(board_to_tuple(next_state), next_player, remaining - 1)
            if suffix is not None:
                return suffix + (col,)

        return None

    result = dfs(board_t, last_player, pieces)
    if result is None:
        return None
    return list(result)


def encode_move_sequence(sequence_cols: Sequence[int]) -> str:
    return "".join(str(c + 1) for c in sequence_cols)


def parse_score_from_solver_output(output: str) -> Optional[int]:
    for raw_line in output.splitlines():
        line = raw_line.strip()
        if not line.startswith("score"):
            continue
        # Example: score = 3 (=)  work = 34
        parts = line.replace("=", " = ").split()
        for i, token in enumerate(parts):
            if token == "=" and i + 1 < len(parts):
                try:
                    return int(parts[i + 1])
                except ValueError:
                    pass
    return None


def evaluate_position_with_solver(solver_path: Path, move_sequence: str, timeout_seconds: float) -> Optional[int]:
    try:
        proc = subprocess.run(
            [str(solver_path)],
            input=(move_sequence + "\n").encode("utf-8"),
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=timeout_seconds,
            check=False,
        )
    except subprocess.TimeoutExpired:
        return None
    except OSError:
        return None

    stdout = proc.stdout.decode("utf-8", errors="replace")
    return parse_score_from_solver_output(stdout)


def choose_move_with_fhourstones(state_payload: Dict[str, Any], worker_state: WorkerState) -> Optional[str]:
    board, turn = extract_board_and_turn(state_payload)
    if board is None or turn is None:
        return None

    sequence = reconstruct_move_sequence(board, turn)
    if sequence is None:
        log("warn", "failed to reconstruct move sequence; using fallback")
        return choose_fallback_move(board)

    base_seq = encode_move_sequence(sequence)
    legal = legal_columns(board)
    if not legal:
        return None

    # Evaluate each candidate by asking solver to score the opponent-to-move position.
    # Lower opponent score is better for us (LOSS < DRAW < WIN from opponent perspective).
    scored: List[Tuple[int, int]] = []  # (score, col)
    for col in legal:
        cand = apply_move(board, col, worker_state.player_id or turn)
        if cand is None:
            continue

        candidate_sequence = base_seq + str(col + 1)
        score = evaluate_position_with_solver(worker_state.solver_path, candidate_sequence, worker_state.solve_timeout_seconds)
        if score is None:
            continue
        scored.append((score, col))

    if not scored:
        log("warn", "solver timed out or produced no score; using fallback")
        return choose_fallback_move(board)

    scored.sort(key=lambda item: (item[0], center_distance(item[1], len(board[0]))))
    chosen_col = scored[0][1]
    return str(chosen_col)


def center_distance(col: int, width: int) -> int:
    return abs(col - (width // 2))


def choose_fallback_move(board: Sequence[Sequence[int]]) -> Optional[str]:
    legal = legal_columns(board)
    if not legal:
        return None
    legal.sort(key=lambda c: center_distance(c, len(board[0])))
    return str(legal[0])


def is_our_turn(state_payload: Dict[str, Any], worker_state: WorkerState) -> bool:
    if "your_turn" in state_payload:
        return bool(state_payload.get("your_turn"))
    turn_player = as_int(state_payload.get("turn_player"), default=0)
    return worker_state.player_id is not None and turn_player == worker_state.player_id


def handle_registered(payload: Dict[str, Any], worker_state: WorkerState) -> None:
    worker_state.session_id = as_int(payload.get("session_id"), default=0)
    log("info", f"registered session_id={worker_state.session_id}")


def handle_manifest(payload: Dict[str, Any], worker_state: WorkerState) -> None:
    worker_state.arena_id = as_int(payload.get("arena_id"), default=0)
    worker_state.player_id = as_int(payload.get("player_id"), default=0)
    worker_state.game = str(payload.get("game") or "")
    log(
        "info",
        f"manifest arena_id={worker_state.arena_id} player_id={worker_state.player_id} game={worker_state.game}",
    )


def handle_state(payload: Dict[str, Any], worker_state: WorkerState) -> None:
    if normalized_game_name(worker_state.game) != "connect4":
        return

    if not is_our_turn(payload, worker_state):
        return

    move = choose_move_with_fhourstones(payload, worker_state)
    if move is None:
        return

    send_message("move", {"move": move})
    log("info", f"submitted move={move}")


def handle_event(payload: Dict[str, Any]) -> None:
    log("info", f"event name={payload.get('name')}")


def ensure_solver_binary(solver_path: Path, source_dir: Path) -> bool:
    if solver_path.exists() and solver_path.is_file():
        return True

    src = source_dir / "SearchGame.c"
    if not src.exists():
        return False

    cmd = ["gcc", "-O2", "-std=c99", str(src), "-o", str(solver_path)]
    try:
        proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
    except OSError:
        return False

    if proc.returncode != 0:
        sys.stderr.write("[fhourstones-worker] failed to build solver binary\n")
        sys.stderr.write(proc.stderr.decode("utf-8", errors="replace") + "\n")
        return False

    return solver_path.exists()


def parse_args(argv: Sequence[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Fhourstones worker for bbs-agent contract")
    parser.add_argument(
        "--solver",
        default="",
        help="Path to fhourstones binary (default: ./fhourstones next to this script)",
    )
    parser.add_argument(
        "--solve-timeout",
        type=float,
        default=1.25,
        help="Per-candidate solver timeout in seconds",
    )
    return parser.parse_args(list(argv))


def main(argv: Sequence[str]) -> int:
    args = parse_args(argv)
    here = Path(__file__).resolve().parent
    solver_path = Path(args.solver).expanduser().resolve() if args.solver else (here / "fhourstones")

    if not ensure_solver_binary(solver_path, here):
        log("error", f"fhourstones binary is unavailable at {solver_path}")

    worker_state = WorkerState(solver_path=solver_path, solve_timeout_seconds=max(0.05, float(args.solve_timeout)))

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

        if msg_type == "hello":
            send_message(
                "hello_ack",
                {
                    "worker_name": "fhourstones_worker",
                    "worker_version": "0.1.0",
                    "language": "python",
                },
                msg.get("id"),
            )
            continue

        if msg_type == "registered":
            handle_registered(payload, worker_state)
            continue

        if msg_type == "manifest":
            handle_manifest(payload, worker_state)
            continue

        if msg_type == "state":
            handle_state(payload, worker_state)
            continue

        if msg_type == "event":
            handle_event(payload)
            continue

        if msg_type == "error":
            log("error", f"agent error: {payload.get('message')}")
            continue

        if msg_type == "shutdown":
            log("info", "shutdown requested")
            return 0

        log("debug", f"ignored message type={msg_type}")

    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
