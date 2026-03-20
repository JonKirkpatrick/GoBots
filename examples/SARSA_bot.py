#!/usr/bin/env python3
"""SARSA bot example for the gridworld_rl plugin via bbs-agent.

This bot talks to the local JSONL bridge (`bbs-agent --listen`) and learns
online with SARSA (on-policy learning).
"""

from __future__ import annotations

import argparse
import json
import random
import socket
import sys
from typing import Any, Dict, List, Optional, Tuple

CONTRACT_VERSION = "0.2"
DEFAULT_ACTIONS = ["up", "down", "left", "right"]


def send_message(writer, msg_type: str, payload: Dict[str, Any]) -> None:
    """Send a properly formatted message to the agent."""
    writer.write(json.dumps({"v": CONTRACT_VERSION, "type": msg_type, "payload": payload}, ensure_ascii=True) + "\n")
    writer.flush()


def log(writer, level: str, message: str) -> None:
    """Send a log message to stderr via the agent."""
    send_message(writer, "log", {"level": level, "message": message})


def normalize_socket_path(raw: str) -> str:
    """Normalize unix socket path."""
    value = raw.strip()
    if value.startswith("unix://"):
        return value[len("unix://") :]
    return value


def as_int(value: Any, default: int = 0) -> int:
    """Parse value as integer safely."""
    if isinstance(value, bool):
        return default
    if isinstance(value, (int, float)):
        return int(value)
    if isinstance(value, str):
        token = value.strip()
        if not token:
            return default
        try:
            return int(token)
        except ValueError:
            return default
    return default


def as_float(value: Any, default: float = 0.0) -> float:
    """Parse value as float safely."""
    if isinstance(value, bool):
        return default
    if isinstance(value, (int, float)):
        return float(value)
    if isinstance(value, str):
        token = value.strip()
        if not token:
            return default
        try:
            return float(token)
        except ValueError:
            return default
    return default


def parse_turn(payload: Dict[str, Any]) -> Tuple[Dict[str, Any], float, bool]:
    """Extract state_obj, reward, done from turn payload robustly."""
    reward = as_float(payload.get("reward"), 0.0)
    done = bool(payload.get("done"))

    obs = payload.get("obs")
    if not isinstance(obs, dict):
        return {}, reward, done

    state_obj = obs.get("state_obj")
    if not isinstance(state_obj, dict):
        raw_state = obs.get("raw_state")
        if isinstance(raw_state, str) and raw_state.strip():
            try:
                parsed = json.loads(raw_state)
                if isinstance(parsed, dict):
                    state_obj = parsed
            except json.JSONDecodeError:
                state_obj = {}

    if not isinstance(state_obj, dict):
        state_obj = {}

    if "reward" in state_obj:
        reward = as_float(state_obj.get("reward"), reward)
    if "done" in state_obj:
        done = bool(state_obj.get("done"))

    return state_obj, reward, done


def state_key_from_state_obj(state_obj: Dict[str, Any]) -> Tuple[int, int]:
    """Extract (row, col) position from state object."""
    pos = state_obj.get("agent")
    if isinstance(pos, dict):
        row = as_int(pos.get("row"), 0)
        col = as_int(pos.get("col"), 0)
    else:
        row = as_int(state_obj.get("row"), 0)
        col = as_int(state_obj.get("col"), 0)
    return (row, col)


def actions_from_state_obj(state_obj: Dict[str, Any]) -> List[str]:
    """Get list of legal actions from state object."""
    legal = state_obj.get("legal_moves")
    if isinstance(legal, list):
        moves = [str(item).strip().lower() for item in legal if str(item).strip()]
        if moves:
            return moves

    all_actions = state_obj.get("all_actions")
    if isinstance(all_actions, list):
        moves = [str(item).strip().lower() for item in all_actions if str(item).strip()]
        if moves:
            return moves

    return list(DEFAULT_ACTIONS)


def is_our_turn(payload: Dict[str, Any], state_obj: Dict[str, Any], player_id: Optional[int]) -> bool:
    """Check if it's our turn to act."""
    obs = payload.get("obs")
    if isinstance(obs, dict) and "your_turn" in obs:
        return bool(obs.get("your_turn"))

    turn_player = as_int(state_obj.get("turn_player"), 0)
    if turn_player == 0 and isinstance(obs, dict):
        turn_player = as_int(obs.get("turn_player"), 0)

    return player_id is not None and turn_player == player_id


class SarsaBot:
    def __init__(self, alpha=0.1, gamma=0.9, epsilon=0.1):
        self.q_table = {}  # (row, col): [q_up, q_right, q_down, q_left]
        self.alpha = alpha
        self.gamma = gamma
        self.epsilon = epsilon
        self.actions = ["up", "right", "down", "left"]
        
        # Track SARSA state
        self.last_state = None
        self.last_action_idx = None

class SarsaBot:
    """SARSA Q-table learner."""
    
    def __init__(self, alpha: float = 0.1, gamma: float = 0.9, epsilon: float = 0.1):
        self.q_table: Dict[Tuple[int, int], List[float]] = {}  # (row, col): [q_up, q_right, q_down, q_left]
        self.alpha = alpha
        self.gamma = gamma
        self.epsilon = epsilon
        self.actions = ["up", "right", "down", "left"]
        
        # Track SARSA state
        self.last_state: Optional[Tuple[int, int]] = None
        self.last_action_idx: Optional[int] = None

    def get_q(self, state: Tuple[int, int]) -> List[float]:
        """Get Q-values for state, initializing if needed."""
        if state not in self.q_table:
            self.q_table[state] = [0.0] * 4
        return self.q_table[state]

    def choose_action(self, state: Tuple[int, int]) -> int:
        """Choose action via epsilon-greedy."""
        if random.random() < self.epsilon:
            return random.randint(0, 3)
        qs = self.get_q(state)
        max_q = max(qs)
        # Random choice among ties
        return random.choice([i for i, q in enumerate(qs) if q == max_q])

    def update(self, s: Tuple[int, int], a_idx: int, r: float, s_next: Tuple[int, int], a_next_idx: int, done: bool) -> None:
        """SARSA update: Q(s,a) += alpha * (r + gamma * Q(s',a') - Q(s,a))."""
        current_q = self.get_q(s)[a_idx]
        if done:
            target = r
        else:
            target = r + self.gamma * self.get_q(s_next)[a_next_idx]
        
        self.q_table[s][a_idx] += self.alpha * (target - current_q)


def main() -> int:
    """Main bot loop."""
    parser = argparse.ArgumentParser(description="SARSA bot for gridworld_rl via bbs-agent")
    parser.add_argument("--socket", default="/tmp/bbs-agent.sock", help="unix socket path")
    parser.add_argument("--name", default="sarsa_explorer", help="bot name used in hello")
    parser.add_argument("--owner-token", default="", help="optional owner token")
    parser.add_argument("--capabilities", default="gridworld_rl", help="comma-separated capabilities")
    parser.add_argument("--bot-id", default="", help="optional existing bot identity id")
    parser.add_argument("--bot-secret", default="", help="optional existing bot identity secret")
    
    parser.add_argument("--alpha", type=float, default=0.1, help="SARSA step size")
    parser.add_argument("--gamma", type=float, default=0.9, help="discount factor")
    parser.add_argument("--epsilon", type=float, default=0.1, help="exploration rate")
    args = parser.parse_args()

    sock_path = normalize_socket_path(args.socket)
    if not sock_path:
        print("socket path is empty", file=sys.stderr)
        return 1

    try:
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.connect(sock_path)
    except Exception as e:
        print(f"failed to connect to {sock_path}: {e}", file=sys.stderr)
        return 1

    reader = sock.makefile("r", encoding="utf-8", newline="\n")
    writer = sock.makefile("w", encoding="utf-8", newline="\n")

    bot = SarsaBot(alpha=args.alpha, gamma=args.gamma, epsilon=args.epsilon)
    
    # Send hello to bbs-agent
    send_message(
        writer,
        "hello",
        {
            "name": args.name,
            "owner_token": args.owner_token,
            "capabilities": [c.strip() for c in args.capabilities.split(",") if c.strip()],
            "bot_id": args.bot_id,
            "bot_secret": args.bot_secret,
        },
    )

    player_id: Optional[int] = None
    prev_state: Optional[Tuple[int, int]] = None
    prev_action_idx: Optional[int] = None
    episodes_completed = 0
    cumulative_reward = 0.0

    try:
        for raw_line in reader:
            line = raw_line.strip()
            if not line:
                continue

            try:
                msg = json.loads(line)
            except json.JSONDecodeError:
                log(writer, "error", f"invalid JSON from agent: {line[:120]}")
                continue

            if not isinstance(msg, dict):
                log(writer, "error", "invalid message envelope")
                continue

            if str(msg.get("v", "")) != CONTRACT_VERSION:
                log(writer, "error", f"unsupported contract version: {msg.get('v')}")
                continue

            msg_type = str(msg.get("type", "")).strip().lower()
            payload = msg.get("payload")
            if not isinstance(payload, dict):
                payload = {}

            # Handle welcome message to capture player_id
            if msg_type == "welcome":
                player_id = as_int(payload.get("player_id"), 0)
                env = str(payload.get("env") or "")
                log(
                    writer,
                    "info",
                    f"ready env={env} player={player_id} arena={payload.get('arena_id')} epsilon={bot.epsilon:.3f}",
                )
                continue

            # Handle turn message (main gameplay loop)
            if msg_type == "turn":
                state_obj, reward, done = parse_turn(payload)
                if not state_obj:
                    continue

                cur_state = state_key_from_state_obj(state_obj)
                cur_actions = actions_from_state_obj(state_obj)
                cumulative_reward += reward

                # SARSA update from previous transition
                cur_action_idx = bot.choose_action(cur_state)
                if prev_state is not None and prev_action_idx is not None:
                    bot.update(prev_state, prev_action_idx, reward, cur_state, cur_action_idx, done)

                if done:
                    episodes_completed += 1
                    if episodes_completed % 10 == 0:
                        avg = cumulative_reward / max(1, episodes_completed)
                        log(
                            writer,
                            "info",
                            f"episodes={episodes_completed} avg_reward={avg:.3f} epsilon={bot.epsilon:.3f}",
                        )
                    prev_state = None
                    prev_action_idx = None
                    cumulative_reward = 0.0
                    continue

                # Check if it's our turn to act
                if not is_our_turn(payload, state_obj, player_id):
                    continue

                # Choose and send action
                prev_state = cur_state
                prev_action_idx = cur_action_idx
                send_message(writer, "action", {"action": bot.actions[cur_action_idx]})
                continue

            # Handle shutdown
            if msg_type == "shutdown":
                log(writer, "info", f"shutdown requested; completed {episodes_completed} episodes")
                return 0

    except KeyboardInterrupt:
        log(writer, "info", "keyboard interrupt")
        return 0
    except Exception as e:
        log(writer, "error", f"unexpected error: {e}")
        return 1
    finally:
        try:
            reader.close()
            writer.close()
            sock.close()
        except Exception:
            pass

    return 0


if __name__ == "__main__":
    raise SystemExit(main())
