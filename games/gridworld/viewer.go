package gridworld

import (
	"encoding/json"
	"fmt"
)

type viewerState struct {
	Map          string   `json:"map"`
	Rows         int      `json:"rows"`
	Cols         int      `json:"cols"`
	Grid         [][]int  `json:"grid"`
	Agent        position `json:"agent"`
	Episode      int      `json:"episode"`
	Episodes     int      `json:"episodes"`
	Step         int      `json:"step"`
	MaxSteps     int      `json:"max_steps"`
	Wins         int      `json:"episode_wins"`
	Losses       int      `json:"episode_losses"`
	Turn         int      `json:"turn"`
	Done         bool     `json:"done"`
	Terminal     string   `json:"terminal,omitempty"`
	LastTerminal string   `json:"last_terminal,omitempty"`
}

type viewerMeta struct {
	MapName       string   `json:"map_name"`
	Grid          [][]int  `json:"grid"`
	AgentPos      position `json:"agent_pos"`
	Episode       int      `json:"episode"`
	EpisodeTotal  int      `json:"episode_total"`
	Step          int      `json:"step"`
	MaxSteps      int      `json:"max_steps"`
	Reward        float64  `json:"reward"`
	Terminal      string   `json:"terminal,omitempty"`
	EpisodeWins   int      `json:"episode_wins"`
	EpisodeLosses int      `json:"episode_losses"`
}

type viewerAdapter struct{}

// SpecFromState converts gridworld state to a visual specification.
func (viewerAdapter) SpecFromState(state string) (map[string]interface{}, error) {
	var payload viewerState
	if err := json.Unmarshal([]byte(state), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse gridworld state: %w", err)
	}

	return map[string]interface{}{
		"game": "gridworld",
		"kind": "gridworld-heatmap",
		"rows": payload.Rows,
		"cols": payload.Cols,
		"player_colors": map[string]string{
			"1": "#0b7285", // agent color (teal)
		},
	}, nil
}

// FrameFromState converts gridworld state to a renderable frame.
func (viewerAdapter) FrameFromState(state string, moveIndex int, timestamp string) (map[string]interface{}, error) {
	var payload viewerState
	if err := json.Unmarshal([]byte(state), &payload); err != nil {
		return nil, fmt.Errorf("failed to parse gridworld state: %w", err)
	}

	// Agent position as a token
	tokens := []map[string]int{
		{
			"player": 1,
			"row":    payload.Agent.Row,
			"col":    payload.Agent.Col,
		},
	}

	// Calculate reward
	reward := 0.0
	if payload.Terminal == "win" {
		reward = 1.0
	} else if payload.Terminal == "loss" {
		reward = -1.0
	}

	// Build gridworld metadata
	meta := viewerMeta{
		MapName:       payload.Map,
		Grid:          payload.Grid,
		AgentPos:      payload.Agent,
		Episode:       payload.Episode,
		EpisodeTotal:  payload.Episodes,
		Step:          payload.Step,
		MaxSteps:      payload.MaxSteps,
		Reward:        reward,
		Terminal:      payload.Terminal,
		EpisodeWins:   payload.Wins,
		EpisodeLosses: payload.Losses,
	}

	// Marshal metadata as JSON and include in RawState for frontend consumption
	metaJSON, _ := json.Marshal(meta)

	return map[string]interface{}{
		"move_index":  moveIndex,
		"turn_player": 1,
		"tokens":      tokens,
		"timestamp":   timestamp,
		"is_terminal": payload.Done,
		"raw_state":   string(metaJSON),
	}, nil
}

func parseGridworldState(state string) (viewerState, error) {
	var parsed viewerState
	if err := json.Unmarshal([]byte(state), &parsed); err != nil {
		return viewerState{}, fmt.Errorf("invalid gridworld state: %w", err)
	}
	return parsed, nil
}

// GetViewerAdapter returns the Gridworld viewer adapter.
func GetViewerAdapter() interface {
	SpecFromState(string) (map[string]interface{}, error)
	FrameFromState(string, int, string) (map[string]interface{}, error)
} {
	return viewerAdapter{}
}
