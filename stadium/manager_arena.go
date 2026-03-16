package stadium

import (
	"errors"
	"fmt"
	"time"

	"github.com/JonKirkpatrick/bbs/games"
)

// StartWatchdog launches a background goroutine that periodically checks all arenas for timeouts and cleans up completed matches.
func (m *Manager) StartWatchdog() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			m.mu.Lock()
			for id, arena := range m.Arenas {

				switch arena.Status {
				case "active":
					maxMoveLimit := arena.MaxMoveLimit()
					if maxMoveLimit <= 0 {
						maxMoveLimit = arena.TimeLimit
					}
					// Active games are strictly timed
					if time.Since(arena.LastMove) > (maxMoveLimit * 3) {
						m.terminateArena(id, "Arena closed: Active game timed out.")
					}
				case "completed":
					// Completed games can linger briefly for stats/spectators
					if time.Since(arena.LastMove) > (1 * time.Minute) {
						m.terminateArena(id, "Arena closed: Match concluded.")
					}
				case "waiting":
					// Waiting arenas can live for an hour
					if time.Since(arena.LastMove) > (1 * time.Hour) {
						m.terminateArena(id, "Arena closed: Lobby timed out.")
					}
				case "aborted":
					// Aborted arenas can live for a short time for debugging
					if time.Since(arena.LastMove) > (5 * time.Minute) {
						m.terminateArena(id, "Arena closed: Match aborted.")
					}
				}
			}
			m.mu.Unlock()
		}
	}()
}

// terminateArena is a helper method to cleanly close an arena and notify participants of the reason.
func (m *Manager) terminateArena(id int, reason string) {
	if arena, ok := m.Arenas[id]; ok {
		arena.NotifyAll("error", reason)
		delete(m.Arenas, id)
		m.broadcastArenaListLocked()
	}
}

// DestroyArena is a public method to forcefully remove an arena, typically called when a player leaves or a match ends.
func (m *Manager) DestroyArena(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Arenas, id)
	m.broadcastArenaListLocked()
}

// NotifyOpponent sends a message to the opponent of the given actorID (1 or 2) in the arena.
func (a *Arena) NotifyOpponent(actorID int, message string) {
	var opponent *Session
	if actorID == 1 {
		opponent = a.Player2
	} else {
		opponent = a.Player1
	}

	if opponent != nil && opponent.Conn != nil {
		fmt.Fprintf(opponent.Conn, "UPDATE: %s\n", message)
	}
}

// NotifyAll sends a message to both players and all observers in the arena.
func (a *Arena) NotifyAll(msgType string, payload interface{}) {
	res := Response{
		Status:  "ok",
		Type:    msgType,
		Payload: payload,
	}

	// Notify Players
	if a.Player1 != nil {
		a.Player1.SendJSON(res)
	}
	if a.Player2 != nil {
		a.Player2.SendJSON(res)
	}

	// Notify Observers
	for _, obs := range a.Observers {
		if obs != nil {
			obs.SendJSON(res)
		}
	}
}

// ListMatches returns a summary of all current arenas, including their ID, game type, player names, and status, for display in the lobby.
func (m *Manager) ListMatches() []ArenaSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.listMatches()
}

// listMatches is an internal method that compiles a list of ArenaSummary structs for all current arenas, used by ListMatches and when broadcasting new arena creation to dashboards.
func (m *Manager) listMatches() []ArenaSummary {
	var list []ArenaSummary
	for id, arena := range m.Arenas {
		summary := ArenaSummary{
			ID:     id,
			Game:   arena.Game.GetName(),
			Status: arena.Status,
		}
		if arena.Player1 != nil {
			summary.P1Name = arena.Player1.BotName
		}
		if arena.Player2 != nil {
			summary.P2Name = arena.Player2.BotName
		}
		list = append(list, summary)
	}
	return list
}

// AddObserver allows a session to start observing an arena, receiving updates without participating as a player.
func (m *Manager) AddObserver(arenaID int, observer *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	arena, exists := m.Arenas[arenaID]
	if !exists {
		return errors.New("arena not found")
	}

	arena.Observers = append(arena.Observers, observer)
	observer.CurrentArena = arena
	m.broadcastArenaListLocked()
	return nil
}

// CreateArena now accepts the fully-initialized GameInstance.
func (m *Manager) CreateArena(game games.GameInstance, gameArgs []string, timeLimit time.Duration, allowHandicap bool) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createArenaLocked(game, gameArgs, timeLimit, allowHandicap)
}

func (m *Manager) createArenaLocked(game games.GameInstance, gameArgs []string, timeLimit time.Duration, allowHandicap bool) int {
	id := m.nextArenaID
	m.nextArenaID++

	m.Arenas[id] = &Arena{
		ID:            id,
		Game:          game,
		GameArgs:      append([]string(nil), gameArgs...),
		TimeLimit:     timeLimit,
		AllowHandicap: allowHandicap,
		Status:        "waiting",
		Observers:     make([]*Session, 0),
		MoveHistory:   make([]MatchMove, 0),
		CreatedAt:     time.Now(),
		LastMove:      time.Now(),
	}
	m.broadcastArenaListLocked()
	return id
}

// JoinArena allows a session to join an existing arena as a player, assigning them to Player 1 or Player 2 as appropriate, and starts the game if both players are present.
func (m *Manager) JoinArena(arenaID int, s *Session, handicap int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.joinArenaLocked(arenaID, s, handicap)
}

func (m *Manager) joinArenaLocked(arenaID int, s *Session, handicap int) error {
	if s == nil || !s.IsRegistered {
		return errors.New("session must be registered before joining an arena")
	}
	if s.CurrentArena != nil {
		return fmt.Errorf("session is already attached to arena %d", s.CurrentArena.ID)
	}

	arena, exists := m.Arenas[arenaID]
	if !exists {
		return errors.New("arena not found")
	}

	appliedHandicap, err := normalizeArenaHandicap(handicap, arena.AllowHandicap)
	if err != nil {
		return err
	}

	if arena.Player1 == nil {
		arena.Player1 = s
		s.PlayerID = 1
		arena.Player1Handicap = appliedHandicap
	} else if arena.Player2 == nil {
		arena.Player2 = s
		s.PlayerID = 2
		arena.Player2Handicap = appliedHandicap
		// Status update happens inside activateArena now
	} else {
		return errors.New("arena full")
	}

	s.CurrentArena = arena

	// 1. Prepare the Payload for the Bot
	// This gives the bot its constraints immediately upon joining.
	manifest := map[string]interface{}{
		"arena_id":                arena.ID,
		"player_id":               s.PlayerID,
		"game":                    arena.Game.GetName(),
		"time_limit_ms":           arena.TimeLimit.Milliseconds(),
		"handicap_enabled":        arena.AllowHandicap,
		"handicap_percent":        appliedHandicap,
		"effective_time_limit_ms": arena.MoveLimitForPlayer(s.PlayerID).Milliseconds(),
	}

	// 2. Send the confirmation to the bot
	s.SendJSON(Response{Status: "ok", Type: "join", Payload: manifest})

	// 3. If the arena is now ready, kick off the game
	if arena.Player1 != nil && arena.Player2 != nil {
		m.activateArena(arena)
	}

	m.broadcastArenaListLocked()
	return nil
}

// HandlePlayerLeave is called when a session disconnects or quits, ensuring that the arena is properly cleaned up and the opponent is notified.
func (m *Manager) HandlePlayerLeave(s *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s.CurrentArena != nil {
		arena := s.CurrentArena

		// If the leaving session is an observer, remove them from the slice and
		// return early — the match itself is unaffected.
		if arena.Player1 != s && arena.Player2 != s {
			for i, obs := range arena.Observers {
				if obs == s {
					arena.Observers = append(arena.Observers[:i], arena.Observers[i+1:]...)
					break
				}
			}
			s.CurrentArena = nil
			m.broadcastArenaListLocked()
			return
		}

		winnerPlayerID := 0

		if arena.Player1 == s && arena.Player2 != nil {
			winnerPlayerID = 2
		}
		if arena.Player2 == s && arena.Player1 != nil {
			winnerPlayerID = 1
		}

		status := "aborted"
		if winnerPlayerID != 0 {
			status = "completed"
		}

		arena.NotifyAll("error", "Player "+s.BotName+" left.")
		_, _ = m.finalizeArenaLocked(arena, "player_left", status, winnerPlayerID, false)
		m.broadcastArenaListLocked()
	}
}

// activateArena sets the arena status to active,
// initializes player time based on handicap settings,
// and notifies both players that the game has started.
func (m *Manager) activateArena(a *Arena) {
	a.Status = "active"
	a.LastMove = time.Now()
	a.ActivatedAt = time.Now()

	// Initialize per-player move clocks from the arena base time and handicap percentages.
	a.Bot1Time = a.MoveLimitForPlayer(1)
	a.Bot2Time = a.MoveLimitForPlayer(2)

	// Notify both bots that the game is ON
	msg := "Game Start! Opponent: " + a.Player1.BotName + " vs " + a.Player2.BotName
	a.Player1.SendJSON(Response{"ok", "info", msg})
	a.Player2.SendJSON(Response{"ok", "info", msg})
}

func normalizeArenaHandicap(handicap int, allowHandicap bool) (int, error) {
	if !allowHandicap {
		if handicap != 0 {
			return 0, errors.New("arena does not allow handicap; use 0")
		}
		return 0, nil
	}

	if handicap < -90 || handicap > 300 {
		return 0, fmt.Errorf("handicap must be between -90 and 300")
	}

	return handicap, nil
}
