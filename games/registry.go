package games

import (
	"fmt"

	"github.com/JonKirkpatrick/bbs/games/connect4"
)

// GameFactory is a function type that returns a new GameInstance
type GameFactory func() GameInstance

// Registry maps game names to their factory functions
var registry = map[string]GameFactory{
	"connect4": func() GameInstance { return connect4.New() },
}

// GetGame creates a new instance of a game by its string name
func GetGame(name string) (GameInstance, error) {
	factory, exists := registry[name]
	if !exists {
		return nil, fmt.Errorf("game '%s' not found in registry", name)
	}
	return factory(), nil
}
