package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/JonKirkpatrick/bbs/games"
	"github.com/JonKirkpatrick/bbs/stadium"
)

type viewerPageData struct {
	ArenaID  int
	MatchID  int
	AdminKey string
}

// viewerParticipant carries player identity and stats for the viewer UI.
type viewerParticipant struct {
	PlayerID int    `json:"player_id"`
	Name     string `json:"name"`
	BotID    string `json:"bot_id,omitempty"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Draws    int    `json:"draws"`
}

type viewerReplayResponse struct {
	MatchID int                 `json:"match_id"`
	Game    string              `json:"game"`
	Spec    games.ViewerSpec    `json:"spec"`
	Frames  []games.ViewerFrame `json:"frames"`
	Players []viewerParticipant `json:"players"`
}

type viewerLiveEvent struct {
	ArenaID int                 `json:"arena_id"`
	Status  string              `json:"status"`
	Spec    games.ViewerSpec    `json:"spec"`
	Frame   games.ViewerFrame   `json:"frame"`
	Players []viewerParticipant `json:"players"`
}

func handleViewerPage(w http.ResponseWriter, r *http.Request) {
	arenaID, _ := parsePositiveQueryInt(r, "arena_id")
	matchID, _ := parsePositiveQueryInt(r, "match_id")

	data := viewerPageData{
		ArenaID:  arenaID,
		MatchID:  matchID,
		AdminKey: strings.TrimSpace(r.URL.Query().Get("admin_key")),
	}

	if err := dashTemplates.ExecuteTemplate(w, "viewer.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleViewerReplayData(w http.ResponseWriter, r *http.Request) {
	matchID, ok := parsePositiveQueryInt(r, "match_id")
	if !ok {
		http.Error(w, "match_id must be a positive integer", http.StatusBadRequest)
		return
	}

	record, exists := stadium.DefaultManager.GetMatchRecord(matchID)
	if !exists {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}

	spec, frames, err := buildReplayFrames(record)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := viewerReplayResponse{
		MatchID: record.MatchID,
		Game:    record.Game,
		Spec:    spec,
		Frames:  frames,
		Players: buildReplayParticipants(record),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func handleViewerLiveSSE(w http.ResponseWriter, r *http.Request) {
	arenaID, ok := parsePositiveQueryInt(r, "arena_id")
	if !ok {
		http.Error(w, "arena_id must be a positive integer", http.StatusBadRequest)
		return
	}

	initial, exists, err := buildLiveViewerEvent(arenaID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !exists {
		http.Error(w, "arena not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	if err := writeViewerSSEJSON(w, "frame", initial); err != nil {
		return
	}

	lastMoveIndex := initial.Frame.MoveIndex
	lastRawState := initial.Frame.RawState
	lastStatus := initial.Status

	sub := stadium.DefaultManager.Subscribe()
	defer stadium.DefaultManager.Unsubscribe(sub)

	for {
		select {
		case <-r.Context().Done():
			return
		case _, alive := <-sub:
			if !alive {
				return
			}

			event, present, buildErr := buildLiveViewerEvent(arenaID)
			if buildErr != nil {
				_ = writeViewerSSEJSON(w, "error", map[string]string{"error": buildErr.Error()})
				return
			}
			if !present {
				_ = writeViewerSSEJSON(w, "closed", map[string]string{"message": "arena no longer active"})
				return
			}

			if event.Frame.MoveIndex == lastMoveIndex && event.Frame.RawState == lastRawState && event.Status == lastStatus {
				continue
			}

			if err := writeViewerSSEJSON(w, "frame", event); err != nil {
				return
			}

			lastMoveIndex = event.Frame.MoveIndex
			lastRawState = event.Frame.RawState
			lastStatus = event.Status
		}
	}
}

func buildReplayFrames(record stadium.MatchRecord) (games.ViewerSpec, []games.ViewerFrame, error) {
	adapter, ok := games.GetViewerAdapter(record.Game)
	if !ok {
		return games.ViewerSpec{}, nil, fmt.Errorf("viewer not available for game %q", record.Game)
	}

	args := append([]string(nil), record.GameArgs...)
	if len(args) == 0 && strings.EqualFold(record.Game, "connect4") {
		if inferred, err := games.InferConnect4ArgsFromState(record.FinalGameState); err == nil {
			args = inferred
		}
	}

	gameName := strings.ToLower(strings.TrimSpace(record.Game))
	game, err := games.GetGame(gameName, args)
	if err != nil {
		return games.ViewerSpec{}, nil, fmt.Errorf("failed to reconstruct game: %w", err)
	}

	initialState := game.GetState()
	spec, err := adapter.SpecFromState(initialState)
	if err != nil {
		return games.ViewerSpec{}, nil, err
	}

	frames := make([]games.ViewerFrame, 0, len(record.Moves)+1)
	initialFrame, err := adapter.FrameFromState(initialState, 0, record.StartedAt)
	if err != nil {
		return games.ViewerSpec{}, nil, err
	}
	frames = append(frames, initialFrame)

	if len(record.Moves) > 0 {
		for i, move := range record.Moves {
			if err := game.ApplyMove(move.PlayerID, move.Move); err != nil {
				return games.ViewerSpec{}, nil, fmt.Errorf("failed to apply move %d (%s): %w", i+1, move.Move, err)
			}
			frame, err := adapter.FrameFromState(game.GetState(), i+1, move.OccurredAt)
			if err != nil {
				return games.ViewerSpec{}, nil, err
			}
			frames = append(frames, frame)
		}
	} else {
		// Backward-compatible fallback for older records lacking per-move player IDs.
		currentPlayer := 1
		for i, move := range record.MoveSequence {
			if err := game.ApplyMove(currentPlayer, move); err != nil {
				return games.ViewerSpec{}, nil, fmt.Errorf("failed to apply fallback move %d (%s): %w", i+1, move, err)
			}
			frame, err := adapter.FrameFromState(game.GetState(), i+1, "")
			if err != nil {
				return games.ViewerSpec{}, nil, err
			}
			frames = append(frames, frame)
			if currentPlayer == 1 {
				currentPlayer = 2
			} else {
				currentPlayer = 1
			}
		}
	}

	if len(frames) > 0 {
		last := &frames[len(frames)-1]
		if record.TerminalStatus == "completed" || record.TerminalStatus == "aborted" {
			last.IsTerminal = true
		}
		if record.IsDraw {
			last.Winner = "draw"
		} else if record.WinnerPlayerID == 1 || record.WinnerPlayerID == 2 {
			last.Winner = fmt.Sprintf("player_%d", record.WinnerPlayerID)
		}
	}

	return spec, frames, nil
}

func buildLiveViewerEvent(arenaID int) (viewerLiveEvent, bool, error) {
	arenaState, exists := stadium.DefaultManager.GetArenaViewerState(arenaID)
	if !exists {
		return viewerLiveEvent{}, false, nil
	}

	adapter, ok := games.GetViewerAdapter(arenaState.Game)
	if !ok {
		return viewerLiveEvent{}, false, fmt.Errorf("viewer not available for game %q", arenaState.Game)
	}

	spec, err := adapter.SpecFromState(arenaState.GameState)
	if err != nil {
		return viewerLiveEvent{}, false, err
	}

	frame, err := adapter.FrameFromState(arenaState.GameState, arenaState.MoveCount, arenaState.LastMoveAt)
	if err != nil {
		return viewerLiveEvent{}, false, err
	}
	if arenaState.Status == "completed" || arenaState.Status == "aborted" {
		frame.IsTerminal = true
		if arenaState.IsDraw {
			frame.Winner = "draw"
		} else if arenaState.WinnerPlayerID == 1 || arenaState.WinnerPlayerID == 2 {
			frame.Winner = fmt.Sprintf("player_%d", arenaState.WinnerPlayerID)
		}
	}

	return viewerLiveEvent{
		ArenaID: arenaState.ArenaID,
		Status:  arenaState.Status,
		Spec:    spec,
		Frame:   frame,
		Players: buildLiveParticipants(arenaState, spec),
	}, true, nil
}

func buildLiveParticipants(arenaState stadium.ArenaViewerState, spec games.ViewerSpec) []viewerParticipant {
	players := make([]viewerParticipant, 0, 2)
	if arenaState.Player1.Name != "" {
		players = append(players, viewerParticipant{
			PlayerID: 1,
			Name:     arenaState.Player1.Name,
			BotID:    arenaState.Player1.BotID,
			Wins:     arenaState.Player1.Wins,
			Losses:   arenaState.Player1.Losses,
			Draws:    arenaState.Player1.Draws,
		})
	}
	if arenaState.Player2.Name != "" {
		players = append(players, viewerParticipant{
			PlayerID: 2,
			Name:     arenaState.Player2.Name,
			BotID:    arenaState.Player2.BotID,
			Wins:     arenaState.Player2.Wins,
			Losses:   arenaState.Player2.Losses,
			Draws:    arenaState.Player2.Draws,
		})
	}
	return players
}

func buildReplayParticipants(record stadium.MatchRecord) []viewerParticipant {
	players := make([]viewerParticipant, 0, 2)
	for _, p := range []struct {
		id  int
		par stadium.MatchParticipant
	}{
		{1, record.Player1},
		{2, record.Player2},
	} {
		if p.par.BotName == "" && p.par.BotID == "" {
			continue
		}
		w, l, d := stadium.DefaultManager.BotStatsForID(p.par.BotID)
		players = append(players, viewerParticipant{
			PlayerID: p.id,
			Name:     p.par.BotName,
			BotID:    p.par.BotID,
			Wins:     w,
			Losses:   l,
			Draws:    d,
		})
	}
	return players
}

func writeViewerSSEJSON(w http.ResponseWriter, eventName string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
		return err
	}

	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

func parsePositiveQueryInt(r *http.Request, key string) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, false
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}
