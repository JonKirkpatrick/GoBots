package stadium

import "time"

// RegistrationResult is returned after a bot registers with the stadium.
type RegistrationResult struct {
	SessionID      int    `json:"session_id"`
	BotID          string `json:"bot_id"`
	BotSecret      string `json:"bot_secret,omitempty"`
	IsNewIdentity  bool   `json:"is_new_identity"`
	Name           string `json:"name"`
	GamesPlayed    int    `json:"games_played"`
	Wins           int    `json:"wins"`
	Losses         int    `json:"losses"`
	Draws          int    `json:"draws"`
	RegisteredAt   string `json:"registered_at"`
	Authentication string `json:"authentication"`
}

// BotProfile stores a bot's persistent identity and long-lived stats.
type BotProfile struct {
	BotID             string
	BotSecret         string
	DisplayName       string
	CreatedAt         time.Time
	LastSeenAt        time.Time
	RegistrationCount int
	GamesPlayed       int
	Wins              int
	Losses            int
	Draws             int
}

// BotProfileSnapshot is a serialized profile for dashboard/state output.
type BotProfileSnapshot struct {
	BotID             string `json:"bot_id"`
	DisplayName       string `json:"display_name"`
	CreatedAt         string `json:"created_at"`
	LastSeenAt        string `json:"last_seen_at"`
	RegistrationCount int    `json:"registration_count"`
	GamesPlayed       int    `json:"games_played"`
	Wins              int    `json:"wins"`
	Losses            int    `json:"losses"`
	Draws             int    `json:"draws"`
}
