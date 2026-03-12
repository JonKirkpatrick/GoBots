```mermaid
sequenceDiagram
    participant B1 as Bot 1
    participant B2 as Bot 2
    participant M as Stadium Manager
    participant G as Game Engine

    B1->>M: JOIN (MySuperBot)
    B2->>M: JOIN (OpponentBot)
    M->>G: InitializeGame()
    G-->>M: Ready
    M->>B1: START_GAME
    M->>B2: START_GAME
    
    loop Game Loop
        M->>B1: REQUEST_MOVE
        B1->>M: MOVE (1,2)
        M->>G: ValidateMove(1,2)
        G-->>M: OK
        M->>G: ApplyMove(1,2)
        M->>B2: UPDATE_STATE
    end
    
    M->>B1: GAME_OVER
    M->>B2: GAME_OVER
```

```mermaid
classDiagram
    class Session {
        +string BotName
        +net.Conn Conn
        +GameInstance CurrentGame
        +SendMessage(msg)
    }

    class GameInstance {
        <<interface>>
        +ValidateMove(move) bool
        +ApplyMove(move)
        +GetState() string
        +IsGameOver() bool
    }

    class Manager {
        +List~Session~ ActiveSessions
        +RegisterBot(Session)
        +StartMatch(Session, Session)
    }

    Manager "1" o-- "*" Session : manages
    Session --> GameInstance : plays    