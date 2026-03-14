# Build-a-Bot Stadium Protocol v1.0

This document defines the communication protocol for the Build-a-Bot Stadium server. All communication is performed over TCP. Responses are sent as JSON objects.

## Connection Lifecycle

1. **Connect**: Open a TCP connection to the stadium server (e.g., `localhost:8080`).
2. **Register**: Send the `REGISTER` command immediately.
3. **Interact**: Send commands and listen for `JSON` responses.
4. **Disconnect**: The server handles cleanup via `QUIT` or an abrupt disconnect.

---

## Command Reference

| Command | Arguments | Description |
| --- | --- | --- |
| `REGISTER` | `<name>` | Authenticates your bot with the server. |
| `CREATE` | `<type> <ms> <v_limit> <handicap>` | Creates a new arena. `v_limit` is the yellow-card threshold. |
| `JOIN` | `<id> <name> <handicap>` | Joins an existing arena by ID. |
| `LIST` | (none) | Lists all currently open arenas. |
| `MOVE` | `<move>` | Submits a move to your active match. |
| `WATCH` | `<id>` | Enters spectator mode for a match. |
| `LEAVE` | (none) | Forfeits current match and exits the arena. |
| `QUIT` | (none) | Closes the connection to the stadium. |

---

## Response Schema

The server communicates status and game data using a standard JSON structure:

```json
{
  "status": "ok | err",
  "type": "register | create | join | move | info | update | error",
  "payload": "string_message_or_data"
}

```

### Example: Successful Move

**Client sends:** `MOVE 3`
**Server responds:**

```json
{"status": "ok", "type": "move", "payload": "accepted"}

```

---

## Gameplay & Enforcement

* **Timeouts**: Moves must be made within the `time_limit` defined at `CREATE`.
* **Yellow Cards**: If `elapsed > time_limit`, the server issues a `warning`. Reaching the `v_limit` results in an automatic `RED CARD` (disqualification).
* **Watchdog**: The server automatically cleans up inactive arenas. `waiting` arenas expire after 1 hour, `active` arenas after 3x their time limit, and `completed` games after 1 minute.
