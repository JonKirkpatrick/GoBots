#!/usr/bin/env python3
"""Minimal Build-a-Bot Stadium Python bot template.

Features:
- Mandatory --server host:port
- Optional --credentials-file for bot_id/bot_secret reuse
- Optional --owner-token to claim dashboard controls
- Auto REGISTER on connect
- Writes assigned credentials for future runs
- Interprets common arena/server JSON responses
"""

from __future__ import annotations

import argparse
import json
import queue
import re
import socket
import threading
from pathlib import Path
from typing import Optional, Tuple


def parse_server_address(raw: str) -> Tuple[str, int]:
    value = raw.strip()
    if not value:
        raise ValueError("server is required")

    if ":" not in value:
        raise ValueError("server must be in host:port format")

    host, port_raw = value.rsplit(":", 1)
    host = host.strip()
    if not host:
        raise ValueError("server host is empty")

    try:
        port = int(port_raw)
    except ValueError as exc:
        raise ValueError("server port must be an integer") from exc

    if port <= 0 or port > 65535:
        raise ValueError("server port must be between 1 and 65535")

    return host, port


def safe_name_for_filename(name: str) -> str:
    cleaned = re.sub(r"[^a-zA-Z0-9_-]+", "_", name.strip())
    cleaned = cleaned.strip("_")
    return cleaned or "bot"


def load_credentials(path: Path) -> Tuple[Optional[str], Optional[str]]:
    if not path.exists():
        return None, None

    bot_id: Optional[str] = None
    bot_secret: Optional[str] = None

    for line in path.read_text(encoding="utf-8").splitlines():
        entry = line.strip()
        if not entry or entry.startswith("#") or "=" not in entry:
            continue
        key, value = entry.split("=", 1)
        key = key.strip().lower()
        value = value.strip()
        if key == "bot_id":
            bot_id = value
        elif key == "bot_secret":
            bot_secret = value

    return bot_id, bot_secret


def save_credentials(path: Path, bot_id: str, bot_secret: str) -> None:
    content = (
        "# Build-a-Bot Stadium bot credentials\n"
        f"bot_id={bot_id}\n"
        f"bot_secret={bot_secret}\n"
    )
    path.write_text(content, encoding="utf-8")


class BotTemplateClient:
    def __init__(self, host: str, port: int) -> None:
        self.host = host
        self.port = port
        self.sock: Optional[socket.socket] = None
        self.reader = None
        self.write_lock = threading.Lock()
        self.stop_event = threading.Event()
        self.register_queue: "queue.Queue[dict]" = queue.Queue(maxsize=1)

    def connect(self) -> None:
        self.sock = socket.create_connection((self.host, self.port), timeout=10)
        self.sock.settimeout(None)
        self.reader = self.sock.makefile("r", encoding="utf-8", newline="\n")

        thread = threading.Thread(target=self._reader_loop, daemon=True)
        thread.start()

    def close(self) -> None:
        self.stop_event.set()
        try:
            if self.reader is not None:
                self.reader.close()
        except Exception:
            pass
        try:
            if self.sock is not None:
                self.sock.close()
        except Exception:
            pass

    def send_json_command(self, msg_type: str, payload: dict) -> None:
        """Send a JSON-envelope formatted command to the server."""
        if self.sock is None:
            raise RuntimeError("socket is not connected")

        envelope = {
            "v": "1",
            "type": msg_type,
            "payload": payload,
        }
        line = json.dumps(envelope, ensure_ascii=True) + "\n"
        with self.write_lock:
            self.sock.sendall(line.encode("utf-8"))

    def send_command(self, command: str) -> None:
        """Send a raw text command to the server (plain text protocol)."""
        if self.sock is None:
            raise RuntimeError("socket is not connected")

        payload = (command.rstrip("\n") + "\n").encode("utf-8")
        with self.write_lock:
            self.sock.sendall(payload)

    def await_register(self, timeout_seconds: float = 10.0) -> dict:
        try:
            return self.register_queue.get(timeout=timeout_seconds)
        except queue.Empty as exc:
            raise RuntimeError("timed out waiting for REGISTER response") from exc

    def _reader_loop(self) -> None:
        assert self.reader is not None
        while not self.stop_event.is_set():
            line = self.reader.readline()
            if line == "":
                print("[socket] disconnected by server")
                return

            text = line.rstrip("\r\n")
            if not text:
                continue

            self._handle_line(text)

    def _handle_line(self, text: str) -> None:
        try:
            msg = json.loads(text)
        except json.JSONDecodeError:
            print(f"[server] {text}")
            return

        if not isinstance(msg, dict):
            print(f"[server-json] {msg}")
            return

        status = str(msg.get("status", "")).lower()
        msg_type = str(msg.get("type", "")).lower()

        if msg_type == "register" and status in {"ok", "err"}:
            if self.register_queue.empty():
                self.register_queue.put(msg)

        self._print_interpreted_message(msg)

    def _print_interpreted_message(self, msg: dict) -> None:
        status = str(msg.get("status", "")).lower()
        msg_type = str(msg.get("type", "")).lower()
        payload = msg.get("payload")

        prefix = f"[{status or 'unknown'}:{msg_type or 'message'}]"

        if msg_type == "create":
            print(f"{prefix} created arena_id={payload}")
            return

        if msg_type == "join" and isinstance(payload, dict):
            arena_id = payload.get("arena_id")
            player_id = payload.get("player_id")
            game = payload.get("game")
            limit = payload.get("effective_time_limit_ms")
            print(
                f"{prefix} joined arena={arena_id} player={player_id} "
                f"game={game} move_limit_ms={limit}"
            )
            return

        if msg_type == "list":
            print(f"{prefix} open arenas:\n{payload}")
            return

        if msg_type == "data":
            if isinstance(payload, str):
                preview = payload if len(payload) <= 280 else payload[:280] + "..."
                print(f"{prefix} state={preview}")
            else:
                print(f"{prefix} state={json.dumps(payload, ensure_ascii=True)}")
            return

        if msg_type == "move":
            print(f"{prefix} move result: {payload}")
            return

        if msg_type == "gameover":
            if isinstance(payload, dict):
                match_id = payload.get("match_id")
                reason = payload.get("end_reason")
                winner = payload.get("winner_bot_name") or payload.get("winner_player_id")
                draw = payload.get("is_draw")
                print(
                    f"{prefix} game over match_id={match_id} reason={reason} "
                    f"winner={winner} draw={draw}"
                )
            else:
                print(f"{prefix} game over payload={payload}")
            return

        if msg_type in {"leave", "update", "info", "timeout", "ejected", "error", "auth"}:
            print(f"{prefix} {payload}")
            return

        if isinstance(payload, (dict, list)):
            print(f"{prefix} {json.dumps(payload, ensure_ascii=True)}")
        else:
            print(f"{prefix} {payload}")


def build_register_command(
    name: str,
    bot_id: Optional[str],
    bot_secret: Optional[str],
    capabilities_csv: str,
    owner_token: Optional[str],
) -> str:
    """Build the text command for REGISTER (plain text protocol)."""
    if any(ch.isspace() for ch in name):
        raise ValueError("bot name cannot contain whitespace for this template")

    parts = [
        "REGISTER",
        name,
        bot_id if bot_id else '""',
        bot_secret if bot_secret else '""',
    ]

    capabilities = [c.strip() for c in capabilities_csv.split(",") if c.strip()]
    if capabilities:
        parts.append(",".join(capabilities))

    if owner_token:
        parts.append(f"owner_token={owner_token.strip()}")

    return " ".join(parts)


def choose_credentials_path(args: argparse.Namespace) -> Path:
    if args.credentials_file:
        return Path(args.credentials_file).expanduser()
    return Path(f"{safe_name_for_filename(args.name)}_credentials.txt")


def run() -> int:
    parser = argparse.ArgumentParser(description="Build-a-Bot Stadium Python bot template")
    parser.add_argument("--server", required=True, help="Server endpoint in host:port format")
    parser.add_argument("--name", default="python_template_bot", help="Bot display name (no spaces)")
    parser.add_argument(
        "--credentials-file",
        default="",
        help="Optional path for bot_id/bot_secret (read if present, write after new registration)",
    )
    parser.add_argument("--owner-token", default="", help="Optional owner_token from dashboard")
    parser.add_argument(
        "--capabilities",
        default="any",
        help="Comma-separated capability tags to advertise during REGISTER",
    )
    args = parser.parse_args()

    host, port = parse_server_address(args.server)
    creds_path = choose_credentials_path(args)

    bot_id: Optional[str] = None
    bot_secret: Optional[str] = None
    loaded_from_file = False

    if args.credentials_file:
        bot_id, bot_secret = load_credentials(creds_path)
        loaded_from_file = bool(bot_id and bot_secret)
        if loaded_from_file:
            print(f"[setup] loaded credentials from {creds_path}")
        else:
            print(f"[setup] credentials file not usable yet: {creds_path}")

    client = BotTemplateClient(host, port)

    try:
        print(f"[setup] connecting to {host}:{port} ...")
        client.connect()

        register_cmd = build_register_command(
            name=args.name,
            bot_id=bot_id,
            bot_secret=bot_secret,
            capabilities_csv=args.capabilities,
            owner_token=args.owner_token,
        )
        print(f"[setup] sending: {register_cmd}")
        client.send_command(register_cmd)

        register_msg = client.await_register(timeout_seconds=12.0)
        if str(register_msg.get("status", "")).lower() != "ok":
            payload = register_msg.get("payload")
            raise RuntimeError(f"register failed: {payload}")

        payload = register_msg.get("payload")
        if isinstance(payload, dict):
            assigned_id = str(payload.get("bot_id") or "").strip()
            assigned_secret = str(payload.get("bot_secret") or "").strip()

            if assigned_id and assigned_secret:
                save_credentials(creds_path, assigned_id, assigned_secret)
                if loaded_from_file:
                    print(f"[setup] refreshed credentials in {creds_path}")
                else:
                    print(f"[setup] saved new credentials to {creds_path}")
            elif not loaded_from_file:
                print("[setup] register succeeded but no new bot_secret was returned")

        print("[setup] connected. Enter commands (LIST, CREATE, JOIN, MOVE, LEAVE, QUIT).")

        while True:
            try:
                command = input("bbs> ").strip()
            except EOFError:
                command = "QUIT"
            except KeyboardInterrupt:
                print("\n[setup] keyboard interrupt -> sending QUIT")
                command = "QUIT"

            if not command:
                continue

            client.send_command(command)
            if command.upper() == "QUIT":
                break

        return 0

    except Exception as exc:
        print(f"[error] {exc}")
        return 1

    finally:
        client.close()


if __name__ == "__main__":
    raise SystemExit(run())
