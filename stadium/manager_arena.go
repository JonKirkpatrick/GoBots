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
			changed := false
			m.mu.Lock()
			for id, arena := range m.Arenas {

				arena.mu.Lock()
				switch arena.Status {
				case "active":
					if games.EnforceMoveClock(arena.Game) {
						maxMoveLimit := arena.MaxMoveLimit()
						if maxMoveLimit <= 0 {
							maxMoveLimit = arena.TimeLimit
						}
						// Timed games are strictly monitored for stale active arenas.
						if time.Since(arena.LastMove) > (maxMoveLimit * 3) {
							arena.mu.Unlock()
							changed = m.terminateArena(id, "Arena closed: Active game timed out.") || changed
							continue
						}
					} else {
						// Untimed environments can run episodic loops; only clean up if idle for a long period.
						if time.Since(arena.LastMove) > (24 * time.Hour) {
							arena.mu.Unlock()
							changed = m.terminateArena(id, "Arena closed: Active arena idle too long.") || changed
							continue
						}
					}
				case "completed":
					// Completed games can linger briefly for stats/spectators
					if time.Since(arena.LastMove) > (1 * time.Minute) {
						arena.mu.Unlock()
						changed = m.terminateArena(id, "Arena closed: Match concluded.") || changed
						continue
					}
				case "waiting":
					// Waiting arenas can live for an hour
					if time.Since(arena.LastMove) > (1 * time.Hour) {
						arena.mu.Unlock()
						changed = m.terminateArena(id, "Arena closed: Lobby timed out.") || changed
						continue
					}
				case "aborted":
					// Aborted arenas can live for a short time for debugging
					if time.Since(arena.LastMove) > (5 * time.Minute) {
						arena.mu.Unlock()
						changed = m.terminateArena(id, "Arena closed: Match aborted.") || changed
						continue
					}
				}
				arena.mu.Unlock()
			}
			m.mu.Unlock()
			if changed {
				m.PublishArenaList()
			}
		}
	}()
}

// terminateArena is a helper method to cleanly close an arena and notify participants of the reason.
func (m *Manager) terminateArena(id int, reason string) bool {
	if arena, ok := m.Arenas[id]; ok {
		arena.NotifyAll("error", reason)
		_ = games.CloseGame(arena.Game)
		delete(m.Arenas, id)
		return true
	}
	return false
}

// DestroyArena is a public method to forcefully remove an arena, typically called when a player leaves or a match ends.
func (m *Manager) DestroyArena(id int) {
	m.mu.Lock()
	if arena, ok := m.Arenas[id]; ok {
		_ = games.CloseGame(arena.Game)
	}
	delete(m.Arenas, id)
	subscribers, events := m.prepareArenaListBroadcastLocked()
	m.mu.Unlock()
	m.publishEvents(subscribers, events)
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
		opponent.SendJSON(Response{Status: "ok", Type: "update", Payload: message})
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
		arena.mu.Lock()
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
		arena.mu.Unlock()
		list = append(list, summary)
	}
	return list
}

// AddObserver allows a session to start observing an arena, receiving updates without participating as a player.
func (m *Manager) AddObserver(arenaID int, observer *Session) error {
	m.mu.Lock()
	arena, exists := m.Arenas[arenaID]
	m.mu.Unlock()
	if !exists {
		return errors.New("arena not found")
	}

	arena.mu.Lock()
	arena.Observers = append(arena.Observers, observer)
	observer.CurrentArena = arena
	arena.mu.Unlock()
	m.PublishArenaList()
	return nil
}

// CreateArena now accepts the fully-initialized GameInstance.
func (m *Manager) CreateArena(game games.GameInstance, gameArgs []string, timeLimit time.Duration, allowHandicap bool) int {
	m.mu.Lock()
	id := m.createArenaLocked(game, gameArgs, timeLimit, allowHandicap)
	subscribers, events := m.prepareArenaListBroadcastLocked()
	m.mu.Unlock()
	m.publishEvents(subscribers, events)
	return id
}

func (m *Manager) createArenaLocked(game games.GameInstance, gameArgs []string, timeLimit time.Duration, allowHandicap bool) int {
	id := m.nextArenaID
	m.nextArenaID++

	moveClockEnabled := games.EnforceMoveClock(game)
	handicapSupported := games.SupportsHandicap(game) && moveClockEnabled
	if !handicapSupported {
		allowHandicap = false
	}

	requiredPlayers := games.RequiredPlayers(game)

	m.Arenas[id] = &Arena{
		ID:                id,
		Game:              game,
		GameArgs:          append([]string(nil), gameArgs...),
		TimeLimit:         timeLimit,
		MoveClockEnabled:  moveClockEnabled,
		HandicapSupported: handicapSupported,
		AllowHandicap:     allowHandicap,
		RequiredPlayers:   requiredPlayers,
		Status:            "waiting",
		Observers:         make([]*Session, 0),
		MoveHistory:       make([]MatchMove, 0),
		CreatedAt:         time.Now(),
		LastMove:          time.Now(),
	}

	if requiredPlayers == 0 {
		m.activateArena(m.Arenas[id])
	}
	return id
}

// JoinArena allows a session to join an existing arena as a player, assigning them to Player 1 or Player 2 as appropriate, and starts the game if both players are present.
func (m *Manager) JoinArena(arenaID int, s *Session, handicap int) error {
	m.mu.Lock()
	arena, exists := m.Arenas[arenaID]
	m.mu.Unlock()
	if !exists {
		return errors.New("arena not found")
	}

	err := m.joinArena(arena, s, handicap)
	if err != nil {
		return err
	}

	m.PublishArenaList()
	return nil
}

func (m *Manager) joinArena(arena *Arena, s *Session, handicap int) error {
	if s == nil || !s.IsRegistered {
		return errors.New("session must be registered before joining an arena")
	}
	if s.CurrentArena != nil {
		return fmt.Errorf("session is already attached to arena %d", s.CurrentArena.ID)
	}

	arena.mu.Lock()
	appliedHandicap, err := normalizeArenaHandicap(handicap, arena.AllowHandicap)
	if err != nil {
		arena.mu.Unlock()
		return err
	}

	requiredPlayers := arena.RequiredPlayers
	if requiredPlayers < 0 || requiredPlayers > 2 {
		requiredPlayers = 2
	}
	if requiredPlayers == 0 {
		arena.mu.Unlock()
		return errors.New("arena does not accept player joins")
	}

	if arena.Player1 == nil {
		arena.Player1 = s
		s.PlayerID = 1
		arena.Player1Handicap = appliedHandicap
	} else if requiredPlayers >= 2 && arena.Player2 == nil {
		arena.Player2 = s
		s.PlayerID = 2
		arena.Player2Handicap = appliedHandicap
		// Status update happens inside activateArena now
	} else {
		arena.mu.Unlock()
		return errors.New("arena full")
	}

	s.CurrentArena = arena
	manifest := map[string]interface{}{
		"arena_id":                arena.ID,
		"player_id":               s.PlayerID,
		"game":                    arena.Game.GetName(),
		"required_players":        requiredPlayers,
		"move_clock_enabled":      arena.MoveClockEnabled,
		"handicap_supported":      arena.HandicapSupported,
		"time_limit_ms":           arena.TimeLimit.Milliseconds(),
		"handicap_enabled":        arena.AllowHandicap,
		"handicap_percent":        appliedHandicap,
		"effective_time_limit_ms": arena.MoveLimitForPlayer(s.PlayerID).Milliseconds(),
	}
	shouldActivate := (requiredPlayers == 1 && arena.Player1 != nil) || (requiredPlayers >= 2 && arena.Player1 != nil && arena.Player2 != nil)
	arena.mu.Unlock()

	// Send join confirmation to the bot.
	s.SendJSON(Response{Status: "ok", Type: "join", Payload: manifest})

	if shouldActivate {
		m.activateArena(arena)
	}

	return nil
}

// HandlePlayerLeave is called when a session disconnects or quits, ensuring that the arena is properly cleaned up and the opponent is notified.
func (m *Manager) HandlePlayerLeave(s *Session) {
	if s == nil || s.CurrentArena == nil {
		return
	}
	arena := s.CurrentArena

	winnerPlayerID := 0
	status := "aborted"

	arena.mu.Lock()
	isObserver := arena.Player1 != s && arena.Player2 != s
	if isObserver {
		for i, obs := range arena.Observers {
			if obs == s {
				arena.Observers = append(arena.Observers[:i], arena.Observers[i+1:]...)
				break
			}
		}
		s.CurrentArena = nil
		arena.mu.Unlock()
		m.PublishArenaList()
		return
	}

	if arena.Player1 == s && arena.Player2 != nil {
		winnerPlayerID = 2
	}
	if arena.Player2 == s && arena.Player1 != nil {
		winnerPlayerID = 1
	}
	if winnerPlayerID != 0 {
		status = "completed"
	}
	arena.mu.Unlock()

	arena.NotifyAll("error", "Player "+s.BotName+" left.")
	_, _ = m.finalizeArenaLocked(arena, "player_left", status, winnerPlayerID, false)
	m.PublishArenaList()
}

// leaveArena removes a session from its current arena without closing the
// TCP connection. The session remains registered and eligible to join another arena.
func (m *Manager) leaveArena(s *Session) bool {
	if s.CurrentArena == nil {
		return false
	}
	arena := s.CurrentArena
	arena.mu.Lock()

	// Observer path — detach without affecting the match itself.
	if arena.Player1 != s && arena.Player2 != s {
		for i, obs := range arena.Observers {
			if obs == s {
				arena.Observers = append(arena.Observers[:i], arena.Observers[i+1:]...)
				break
			}
		}
		s.CurrentArena = nil
		arena.mu.Unlock()
		return true
	}

	// Waiting arena (at most one player occupying a slot) — detach cleanly so
	// the slot can be claimed by another bot without finalizing the arena.
	if arena.Status == "waiting" {
		if arena.Player1 == s {
			arena.Player1 = nil
		} else {
			arena.Player2 = nil
		}
		s.CurrentArena = nil
		s.PlayerID = 0
		arena.mu.Unlock()
		s.SendJSON(Response{Status: "ok", Type: "leave", Payload: "Left arena successfully"})
		return true
	}

	// Active arena — the leaving player forfeits; finalize and notify.
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
	arena.mu.Unlock()
	arena.NotifyAll("info", "Player "+s.BotName+" left the arena.")
	_, _ = m.finalizeArenaLocked(arena, "player_left", status, winnerPlayerID, false)
	// finalizeArenaLocked clears CurrentArena and PlayerID for both players.
	return true
}

// LeaveArena removes a session from its current arena without ejecting it from
// the stadium. The session stays registered and can join another arena.
func (m *Manager) LeaveArena(s *Session) {
	if !m.leaveArena(s) {
		return
	}
	m.PublishArenaList()
}

// LeaveArenaForSession removes the session with the given ID from its current
// arena without closing its TCP connection. Used by the admin dashboard.
func (m *Manager) LeaveArenaForSession(sessionID int) error {
	m.mu.Lock()
	sess, exists := m.ActiveSessions[sessionID]
	m.mu.Unlock()

	if !exists {
		return errors.New("session not found")
	}
	if sess.CurrentArena == nil {
		return errors.New("session is not currently in an arena")
	}
	if m.leaveArena(sess) {
		m.PublishArenaList()
	}
	return nil
}

// JoinArenaForSession joins the session with the given ID to an arena on behalf
// of the admin — the internal equivalent of the JOIN TCP command.
func (m *Manager) JoinArenaForSession(sessionID int, arenaID int, handicap int) error {
	m.mu.Lock()
	sess, exists := m.ActiveSessions[sessionID]
	arena, arenaExists := m.Arenas[arenaID]
	m.mu.Unlock()

	if !exists {
		return errors.New("session not found")
	}
	if !arenaExists {
		return errors.New("arena not found")
	}
	err := m.joinArena(arena, sess, handicap)
	if err != nil {
		return err
	}
	m.PublishArenaList()
	return nil
}

// activateArena sets the arena status to active,
// initializes player time based on handicap settings,
// and notifies both players that the game has started.
func (m *Manager) activateArena(a *Arena) {
	a.mu.Lock()
	a.Status = "active"
	a.LastMove = time.Now()
	a.ActivatedAt = time.Now()

	// Initialize per-player move clocks from the arena base time and handicap percentages.
	a.Bot1Time = a.MoveLimitForPlayer(1)
	a.Bot2Time = a.MoveLimitForPlayer(2)
	a.mu.Unlock()

	if a.Player2 != nil {
		msg := "Game Start! Opponent: " + a.Player1.BotName + " vs " + a.Player2.BotName
		a.Player1.SendJSON(Response{"ok", "info", msg})
		a.Player2.SendJSON(Response{"ok", "info", msg})
	} else if a.Player1 != nil {
		a.Player1.SendJSON(Response{"ok", "info", "Game Start! Solo environment active."})
	}

	// Send the initial game state so bots receive the same "data" message
	// they would get after any move — this lets player 1's bot know it's
	// their turn immediately without waiting for a human trigger.
	a.NotifyAll("data", a.Game.GetState())
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
