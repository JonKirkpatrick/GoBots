package stadium

import "time"

// ArenaViewerState is a lock-safe snapshot used by live viewer endpoints.
type ArenaViewerState struct {
	ArenaID        int
	Game           string
	GameState      string
	MoveCount      int
	Status         string
	LastMoveAt     string
	WinnerPlayerID int
	IsDraw         bool
}

// GetArenaViewerState returns the current renderable state for one arena.
func (m *Manager) GetArenaViewerState(arenaID int) (ArenaViewerState, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	arena, exists := m.Arenas[arenaID]
	if !exists || arena == nil || arena.Game == nil {
		return ArenaViewerState{}, false
	}

	return ArenaViewerState{
		ArenaID:        arena.ID,
		Game:           arena.Game.GetName(),
		GameState:      arena.Game.GetState(),
		MoveCount:      len(arena.MoveHistory),
		Status:         arena.Status,
		LastMoveAt:     arena.LastMove.UTC().Format(time.RFC3339Nano),
		WinnerPlayerID: arena.WinnerPlayerID,
		IsDraw:         arena.IsDraw,
	}, true
}
