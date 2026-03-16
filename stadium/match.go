package stadium

// MatchMove captures one move in chronological order for a match.
type MatchMove struct {
	Number     int    `json:"number"`
	PlayerID   int    `json:"player_id"`
	SessionID  int    `json:"session_id"`
	BotID      string `json:"bot_id"`
	BotName    string `json:"bot_name"`
	Move       string `json:"move"`
	ElapsedMS  int64  `json:"elapsed_ms"`
	OccurredAt string `json:"occurred_at"`
}

// MatchParticipant is the participant metadata captured in match records.
type MatchParticipant struct {
	SessionID    int      `json:"session_id"`
	BotID        string   `json:"bot_id"`
	BotName      string   `json:"bot_name"`
	Capabilities []string `json:"capabilities"`
	RemoteAddr   string   `json:"remote_addr"`
}

// MatchRecord is the terminal record for one arena lifecycle.
type MatchRecord struct {
	MatchID        int                `json:"match_id"`
	ArenaID        int                `json:"arena_id"`
	Game           string             `json:"game"`
	GameArgs       []string           `json:"game_args,omitempty"`
	TerminalStatus string             `json:"terminal_status"`
	EndReason      string             `json:"end_reason"`
	WinnerPlayerID int                `json:"winner_player_id"`
	WinnerBotID    string             `json:"winner_bot_id"`
	WinnerBotName  string             `json:"winner_bot_name"`
	IsDraw         bool               `json:"is_draw"`
	StartedAt      string             `json:"started_at"`
	EndedAt        string             `json:"ended_at"`
	Player1        MatchParticipant   `json:"player1"`
	Player2        MatchParticipant   `json:"player2"`
	Observers      []ObserverSnapshot `json:"observers"`
	MoveCount      int                `json:"move_count"`
	MoveSequence   []string           `json:"move_sequence"`
	CompactMoves   string             `json:"compact_moves"`
	Moves          []MatchMove        `json:"moves"`
	FinalGameState string             `json:"final_game_state"`
}
