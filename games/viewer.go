package games

import (
	"strings"

	"github.com/JonKirkpatrick/bbs/games/connect4"
	"github.com/JonKirkpatrick/bbs/games/gridworld"
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

// LiveViewerProvider is an optional extension for game instances that can
// provide render-ready viewer payloads directly from in-memory game state.
// Process plugins can implement this over RPC to define custom visuals.
type LiveViewerProvider interface {
	ViewerSpec() (ViewerSpec, error)
	ViewerFrame(moveIndex int, timestamp string) (ViewerFrame, error)
}

// rawStateAdapter is a safe fallback for games without a custom renderer.
// It preserves access to raw state in the viewer while signaling that no
// structured board renderer is available yet.
type rawStateAdapter struct {
	gameName string
}

func (a rawStateAdapter) SpecFromState(_ string) (ViewerSpec, error) {
	return ViewerSpec{
		Game: a.gameName,
		Kind: "raw-state",
		Rows: 1,
		Cols: 1,
	}, nil
}

func (a rawStateAdapter) FrameFromState(state string, moveIndex int, timestamp string) (ViewerFrame, error) {
	return ViewerFrame{
		MoveIndex: moveIndex,
		Timestamp: timestamp,
		RawState:  state,
	}, nil
}

// adapterWrapper wraps game-specific adapters to satisfy ViewerAdapter interface.
type adapterWrapper struct {
	rawAdapter interface {
		SpecFromState(string) (map[string]interface{}, error)
		FrameFromState(string, int, string) (map[string]interface{}, error)
	}
}

func (w adapterWrapper) SpecFromState(state string) (ViewerSpec, error) {
	raw, err := w.rawAdapter.SpecFromState(state)
	if err != nil {
		return ViewerSpec{}, err
	}
	return mapToViewerSpec(raw), nil
}

func (w adapterWrapper) FrameFromState(state string, moveIndex int, timestamp string) (ViewerFrame, error) {
	raw, err := w.rawAdapter.FrameFromState(state, moveIndex, timestamp)
	if err != nil {
		return ViewerFrame{}, err
	}
	return mapToViewerFrame(raw), nil
}

// mapToViewerSpec converts a map to ViewerSpec.
func mapToViewerSpec(m map[string]interface{}) ViewerSpec {
	spec := ViewerSpec{}
	if v, ok := m["game"].(string); ok {
		spec.Game = v
	}
	if v, ok := m["kind"].(string); ok {
		spec.Kind = v
	}
	if v, ok := m["rows"].(int); ok {
		spec.Rows = v
	}
	if v, ok := m["cols"].(int); ok {
		spec.Cols = v
	}
	if v, ok := m["player_colors"].(map[string]string); ok {
		spec.PlayerColors = v
	}
	return spec
}

// mapToViewerFrame converts a map to ViewerFrame.
func mapToViewerFrame(m map[string]interface{}) ViewerFrame {
	frame := ViewerFrame{}
	if v, ok := m["move_index"].(int); ok {
		frame.MoveIndex = v
	}
	if v, ok := m["turn_player"].(int); ok {
		frame.TurnPlayer = v
	}
	if v, ok := m["tokens"].([]map[string]int); ok {
		frame.Tokens = make([]ViewerToken, len(v))
		for i, token := range v {
			frame.Tokens[i] = ViewerToken{
				Player: token["player"],
				Row:    token["row"],
				Col:    token["col"],
			}
		}
	}
	if v, ok := m["timestamp"].(string); ok {
		frame.Timestamp = v
	}
	if v, ok := m["is_terminal"].(bool); ok {
		frame.IsTerminal = v
	}
	if v, ok := m["winner"].(string); ok {
		frame.Winner = v
	}
	if v, ok := m["raw_state"].(string); ok {
		frame.RawState = v
	}
	return frame
}

// GetViewerAdapter resolves a viewer adapter by game name.
func GetViewerAdapter(gameName string) (ViewerAdapter, bool) {
	normalized := strings.ToLower(strings.TrimSpace(gameName))
	switch normalized {
	case "connect4":
		return adapterWrapper{connect4.GetViewerAdapter()}, true
	case "gridworld":
		return adapterWrapper{gridworld.GetViewerAdapter()}, true
	default:
		if normalized == "" {
			return nil, false
		}
		return rawStateAdapter{gameName: normalized}, true
	}
}

// InferConnect4ArgsFromState extracts rows/cols from serialized connect4 state.
func InferConnect4ArgsFromState(state string) ([]string, error) {
	return connect4.InferArgsFromState(state)
}
