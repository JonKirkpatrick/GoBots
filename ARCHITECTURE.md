```mermaid
sequenceDiagram
    participant B1 as Player Bot 1
    participant B2 as Player Bot 2
    participant S as Spectator
    participant M as Stadium Manager
    participant A as Arena (Match 1)

    Note over B1, B2: Auth Phase
    B1->>M: REGISTER Bot_A
    M-->>B1: OK (SessionID: 1)
    B2->>M: REGISTER Bot_B
    M-->>B2: OK (SessionID: 2)

    Note over B1, A: Setup Phase
    B1->>M: CREATE connect4 1000 3 true
    M->>A: New Arena(1, Time:1000ms, VLimit:3)
    M-->>B1: OK (ArenaID: 1)

    B2->>M: JOIN 1 Bot_B 0
    M->>A: Set Player 2 (Bot_B)
    A->>A: Status = "active"
    A-->>B1: INFO: Game Start! Opponent: Bot_B
    A-->>B2: INFO: Game Start! Opponent: Bot_A

    Note over B1, S: Play & Watch
    S->>M: WATCH 1
    B1->>A: MOVE 3
    alt Move Within Time
        A->>A: Reset LastMove Timer
        A-->>B2: UPDATE: P1 moved to 3
        A-->>S: DATA: (Current Board State)
    else Move Exceeds 1000ms
        A->>A: Violations[P1]++
        A-->>B1: WARNING: Yellow Card (1/3)
    end
```

```mermaid
classDiagram
    class Session {
        +int SessionID
        +string BotName
        +bool IsRegistered
        +int PlayerID
        +Arena CurrentArena
        +net.Conn Conn
        +SendJSON(Response)
    }

    class Arena {
        +int ArenaID
        +string Status
        +Session Player1
        +Session Player2
        +List~Session~ Observers
        +GameInstance Game
        +Duration TimeLimit
        +int ViolationLimit
        +Map Violations
        +Time LastMove
        +NotifyAll(type, payload)
    }

    class Manager {
        +Map ActiveSessions
        +Map Arenas
        +int nextSessionID
        +RegisterSession(Session, name)
        +CreateArena(type, limit, vLimit, handicap)
        +JoinArena(id, Session, handicap)
        +startWatchdog()
    }

    Manager "1" *-- "*" Arena : manages
    Manager "1" *-- "*" Session : tracks
    Arena "1" o-- "2" Session : participants
    Arena "1" o-- "*" Session : observers
    Session "0..1" -- "0..1" Arena : linked to
