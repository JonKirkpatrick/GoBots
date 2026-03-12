package connect4

import (
	"errors"
	"fmt"
)

type Connect4Game struct {
	Board [6][7]int
	Turn  int // 1 or 2
}

func (c *Connect4Game) GetName() string {
	return "Connect4"
}

func (c *Connect4Game) GetState() string {
	return fmt.Sprintf("Board state: %v, Turn: Player %d", c.Board, c.Turn)
}

func (c *Connect4Game) ValidateMove(playerID int, move string) error {
	if playerID != c.Turn {
		return errors.New("not your turn")
	}
	// Add logic to check if column is full
	return nil
}

func (c *Connect4Game) ApplyMove(playerID int, move string) error {
	// Add logic to update c.Board and switch c.Turn
	return nil
}

func (c *Connect4Game) IsGameOver() (bool, string) {
	// Add win-condition logic here
	return false, ""
}
