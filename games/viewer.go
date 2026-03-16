package games

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ViewerSpec describes a game board in renderer-friendly terms.
type ViewerSpec struct {
	Game         string            `json:"game"`
	Kind         string            `json:"kind"`
	Rows         int               `json:"rows"`
	Cols         int               `json:"cols"`
	PlayerColors map[string]string `json:"player_colors,omitempty"`
}

// ViewerToken is a positioned game token on the board.
type ViewerToken struct {
	Player int `json:"player"`
	Row    int `json:"row"`
	Col    int `json:"col"`
}

// ViewerFrame is a renderable state snapshot at a given move index.
type ViewerFrame struct {
	MoveIndex  int           `json:"move_index"`
	TurnPlayer int           `json:"turn_player"`
	Tokens     []ViewerToken `json:"tokens"`
	Timestamp  string        `json:"timestamp,omitempty"`
	IsTerminal bool          `json:"is_terminal"`
	Winner     string        `json:"winner,omitempty"`
	RawState   string        `json:"raw_state,omitempty"`
}

// ViewerAdapter converts game state strings into a visual spec and frame data.
type ViewerAdapter interface {
	SpecFromState(state string) (ViewerSpec, error)
	FrameFromState(state string, moveIndex int, timestamp string) (ViewerFrame, error)
}

// GetViewerAdapter resolves a viewer adapter by game name.
func GetViewerAdapter(gameName string) (ViewerAdapter, bool) {
	switch strings.ToLower(strings.TrimSpace(gameName)) {
	case "connect4":
		return connect4ViewerAdapter{}, true
	default:
		return nil, false
	}
}

// InferConnect4ArgsFromState extracts rows/cols from serialized connect4 state.
func InferConnect4ArgsFromState(state string) ([]string, error) {
	parsed, err := parseConnect4State(state)
	if err != nil {
		return nil, err
	}
	if len(parsed.Board) == 0 || len(parsed.Board[0]) == 0 {
		return nil, fmt.Errorf("connect4 state board is empty")
	}

	return []string{
		strconv.Itoa(len(parsed.Board)),
		strconv.Itoa(len(parsed.Board[0])),
	}, nil
}

type connect4State struct {
	Board [][]int `json:"board"`
	Turn  int     `json:"turn"`
}

type connect4ViewerAdapter struct{}

func (connect4ViewerAdapter) SpecFromState(state string) (ViewerSpec, error) {
	parsed, err := parseConnect4State(state)
	if err != nil {
		return ViewerSpec{}, err
	}

	rows := len(parsed.Board)
	cols := 0
	if rows > 0 {
		cols = len(parsed.Board[0])
	}

	return ViewerSpec{
		Game: "connect4",
		Kind: "connect4-grid",
		Rows: rows,
		Cols: cols,
		PlayerColors: map[string]string{
			"1": "#ef476f",
			"2": "#ffd166",
		},
	}, nil
}

func (connect4ViewerAdapter) FrameFromState(state string, moveIndex int, timestamp string) (ViewerFrame, error) {
	parsed, err := parseConnect4State(state)
	if err != nil {
		return ViewerFrame{}, err
	}

	tokens := make([]ViewerToken, 0)
	for r := range parsed.Board {
		for c := range parsed.Board[r] {
			player := parsed.Board[r][c]
			if player == 0 {
				continue
			}
			tokens = append(tokens, ViewerToken{Player: player, Row: r, Col: c})
		}
	}

	return ViewerFrame{
		MoveIndex:  moveIndex,
		TurnPlayer: parsed.Turn,
		Tokens:     tokens,
		Timestamp:  timestamp,
		RawState:   state,
	}, nil
}

func parseConnect4State(state string) (connect4State, error) {
	var parsed connect4State
	if err := json.Unmarshal([]byte(state), &parsed); err != nil {
		return connect4State{}, fmt.Errorf("invalid connect4 state: %w", err)
	}
	return parsed, nil
}
