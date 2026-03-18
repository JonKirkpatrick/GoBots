package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/JonKirkpatrick/bbs/games/pluginapi"
)

type counterGame struct {
	target int
	value  int
	turn   int
	over   bool
}

type counterState struct {
	Target int  `json:"target"`
	Value  int  `json:"value"`
	Turn   int  `json:"turn"`
	Done   bool `json:"done"`
}

func newCounterGame(args []string) (pluginapi.Game, error) {
	target := 10
	for _, raw := range args {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		if !strings.Contains(part, "=") {
			if parsed, err := strconv.Atoi(part); err == nil && parsed >= 3 {
				target = parsed
			}
			continue
		}

		kv := strings.SplitN(part, "=", 2)
		key := strings.ToLower(strings.TrimSpace(kv[0]))
		val := strings.TrimSpace(kv[1])
		if key == "target" {
			parsed, err := strconv.Atoi(val)
			if err != nil || parsed < 3 {
				return nil, errors.New("target must be an integer >= 3")
			}
			target = parsed
		}
	}

	return &counterGame{target: target, value: 0, turn: 1}, nil
}

func (g *counterGame) GetName() string {
	return "counter"
}

func (g *counterGame) GetState() string {
	payload := counterState{Target: g.target, Value: g.value, Turn: g.turn, Done: g.over}
	encoded, _ := json.Marshal(payload)
	return string(encoded)
}

func (g *counterGame) ValidateMove(playerID int, move string) error {
	if g.over {
		return errors.New("game already ended")
	}
	if playerID != g.turn {
		return errors.New("not your turn")
	}
	move = strings.TrimSpace(move)
	if move != "1" {
		return errors.New("only move 1 is allowed")
	}
	return nil
}

func (g *counterGame) ApplyMove(playerID int, move string) error {
	if err := g.ValidateMove(playerID, move); err != nil {
		return err
	}

	g.value++
	if g.value >= g.target {
		g.over = true
		return nil
	}
	if g.turn == 1 {
		g.turn = 2
	} else {
		g.turn = 1
	}
	return nil
}

func (g *counterGame) IsGameOver() (bool, string) {
	if !g.over {
		return false, ""
	}
	return true, fmt.Sprintf("Player %d", g.turn)
}

func main() {
	if err := pluginapi.Serve(newCounterGame); err != nil {
		fmt.Fprintln(os.Stderr, "counter plugin error:", err)
		os.Exit(1)
	}
}
