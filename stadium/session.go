package stadium

import (
	"net"
	"time"

	"github.com/JonKirkpatrick/bbs/games"
)

// Session represents an individual bot's connection and state within the Build-a-Bot Stadium.
type Session struct {
	SessionID    int                // Unique identifier for this session
	Conn         net.Conn           // The TCP connection to the bot
	BotName      string             // The name of the bot (set during registration)
	Game         games.GameInstance // The link to the "Rulebook"
	PlayerID     int                // 1 or 2, assigned when match starts
	CurrentArena *Arena             // The arena this session is currently in
	Capabilities []string           // Optional: List of game types or features the bot supports
	IsRegistered bool               // Whether the bot has completed registration
}

// BotSettings represents the configuration options for a bot when creating or joining an arena, such as time limits and handicap settings.
type BotSettings struct {
	TimeLimit time.Duration // Time limit per move
	Handicap  int           // Handicap value, if applicable
}
