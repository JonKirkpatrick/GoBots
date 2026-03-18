package games

// GameCloser is an optional extension for game implementations that need cleanup.
type GameCloser interface {
	Close() error
}

// CloseGame performs optional cleanup for game instances that hold external resources.
func CloseGame(game GameInstance) error {
	if game == nil {
		return nil
	}
	if closer, ok := game.(GameCloser); ok {
		return closer.Close()
	}
	return nil
}
