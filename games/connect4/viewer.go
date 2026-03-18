package connect4

import (
	"encoding/json"
	"fmt"
	"strconv"
)

type connect4State struct {
	Board [][]int `json:"board"`
	Turn  int     `json:"turn"`
}

type viewerAdapter struct{}

// SpecFromState converts connect4 state to a visual specification.
func (viewerAdapter) SpecFromState(state string) (map[string]interface{}, error) {
	parsed, err := parseConnect4State(state)
	if err != nil {
		return nil, err
	}

	rows := len(parsed.Board)
	cols := 0
	if rows > 0 {
		cols = len(parsed.Board[0])
	}

	return map[string]interface{}{
		"game": "connect4",
		"kind": "connect4-grid",
		"rows": rows,
		"cols": cols,
		"player_colors": map[string]string{
			"1": "#ef476f",
			"2": "#ffd166",
		},
	}, nil
}

// FrameFromState converts connect4 state to a renderable frame.
func (viewerAdapter) FrameFromState(state string, moveIndex int, timestamp string) (map[string]interface{}, error) {
	parsed, err := parseConnect4State(state)
	if err != nil {
		return nil, err
	}

	tokens := make([]map[string]int, 0)
	for r := range parsed.Board {
		for c := range parsed.Board[r] {
			player := parsed.Board[r][c]
			if player == 0 {
				continue
			}
			tokens = append(tokens, map[string]int{
				"player": player,
				"row":    r,
				"col":    c,
			})
		}
	}

	return map[string]interface{}{
		"move_index":  moveIndex,
		"turn_player": parsed.Turn,
		"tokens":      tokens,
		"timestamp":   timestamp,
		"raw_state":   state,
	}, nil
}

func parseConnect4State(state string) (connect4State, error) {
	var parsed connect4State
	if err := json.Unmarshal([]byte(state), &parsed); err != nil {
		return connect4State{}, fmt.Errorf("invalid connect4 state: %w", err)
	}
	return parsed, nil
}

// GetViewerAdapter returns the Connect4 viewer adapter.
func GetViewerAdapter() interface {
	SpecFromState(string) (map[string]interface{}, error)
	FrameFromState(string, int, string) (map[string]interface{}, error)
} {
	return viewerAdapter{}
}

// InferArgsFromState extracts rows/cols from serialized connect4 state to support reconstruction.
func InferArgsFromState(state string) ([]string, error) {
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
