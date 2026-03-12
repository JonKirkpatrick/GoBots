package connect4

import "github.com/JonKirkpatrick/bbs/games"

// New creates a new Connect4 game that satisfies the GameInstance interface
func New() games.GameInstance {
	return &Connect4Game{} // Assuming you have a struct named Connect4Game
}
