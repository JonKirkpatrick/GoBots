package games

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/JonKirkpatrick/bbs/games/connect4"
	"github.com/JonKirkpatrick/bbs/games/gridworld"
)

// GameFactory now receives the remainder of the command line parts
type GameFactory func(args []string) (GameInstance, error)

// GameArgSpec describes one optional/required argument for arena creation.
type GameArgSpec struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	InputType    string `json:"input_type"`
	Placeholder  string `json:"placeholder,omitempty"`
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required,omitempty"`
	Help         string `json:"help,omitempty"`
}

// GameCatalogEntry describes compile-time game metadata for dashboard UIs.
type GameCatalogEntry struct {
	Name              string        `json:"name"`
	DisplayName       string        `json:"display_name"`
	Args              []GameArgSpec `json:"args,omitempty"`
	SupportsMoveClock bool          `json:"supports_move_clock"`
	SupportsHandicap  bool          `json:"supports_handicap"`
}

type gameRegistration struct {
	Factory GameFactory
	Catalog GameCatalogEntry
}

func GetGame(name string, args []string) (GameInstance, error) {
	lookupName := strings.ToLower(strings.TrimSpace(name))
	registrations := allRegistrations()
	registration, exists := registrations[lookupName]
	if !exists {
		return nil, fmt.Errorf("game '%s' not found", lookupName)
	}
	return registration.Factory(args)
}

// AvailableGameCatalog returns a stable, sorted list of compile-time game metadata.
func AvailableGameCatalog() []GameCatalogEntry {
	registrations := allRegistrations()
	names := make([]string, 0, len(registrations))
	for name := range registrations {
		names = append(names, name)
	}
	sort.Strings(names)

	entries := make([]GameCatalogEntry, 0, len(names))
	for _, name := range names {
		entry := registrations[name].Catalog
		if len(entry.Args) > 0 {
			argsCopy := make([]GameArgSpec, len(entry.Args))
			copy(argsCopy, entry.Args)
			entry.Args = argsCopy
		}
		entries = append(entries, entry)
	}

	return entries
}

// registry maps game names to their corresponding factory functions
// for dynamic instantiation.
var builtinRegistry = map[string]gameRegistration{
	"connect4": {
		Factory: func(args []string) (GameInstance, error) {
			rows, cols := 6, 7 // Defaults
			positional := make([]string, 0, 2)

			for _, raw := range args {
				part := strings.TrimSpace(raw)
				if part == "" {
					continue
				}

				if strings.Contains(part, "=") {
					kv := strings.SplitN(part, "=", 2)
					key := strings.ToLower(strings.TrimSpace(kv[0]))
					val := strings.TrimSpace(kv[1])

					switch key {
					case "rows":
						parsed, err := strconv.Atoi(val)
						if err != nil || parsed <= 0 {
							return nil, errors.New("invalid rows")
						}
						rows = parsed
					case "cols", "columns":
						parsed, err := strconv.Atoi(val)
						if err != nil || parsed <= 0 {
							return nil, errors.New("invalid columns")
						}
						cols = parsed
					}

					continue
				}

				positional = append(positional, part)
			}

			if len(positional) >= 1 {
				parsed, err := strconv.Atoi(positional[0])
				if err != nil || parsed <= 0 {
					return nil, errors.New("invalid rows")
				}
				rows = parsed
			}

			if len(positional) >= 2 {
				parsed, err := strconv.Atoi(positional[1])
				if err != nil || parsed <= 0 {
					return nil, errors.New("invalid columns")
				}
				cols = parsed
			}
			return connect4.New(rows, cols), nil
		},
		Catalog: GameCatalogEntry{
			Name:              "connect4",
			DisplayName:       "Connect4",
			SupportsMoveClock: true,
			SupportsHandicap:  true,
			Args: []GameArgSpec{
				{
					Key:          "rows",
					Label:        "Rows",
					InputType:    "number",
					DefaultValue: "6",
					Required:     true,
					Help:         "Board height.",
				},
				{
					Key:          "cols",
					Label:        "Columns",
					InputType:    "number",
					DefaultValue: "7",
					Required:     true,
					Help:         "Board width.",
				},
			},
		},
	},
	"gridworld": {
		Factory: func(args []string) (GameInstance, error) {
			return gridworld.New(args)
		},
		Catalog: GameCatalogEntry{
			Name:              "gridworld",
			DisplayName:       "Gridworld",
			SupportsMoveClock: false,
			SupportsHandicap:  false,
			Args: []GameArgSpec{
				{
					Key:          "map",
					Label:        "Map",
					InputType:    "text",
					DefaultValue: "default",
					Help:         "Map name in maps/gridworld/.",
				},
				{
					Key:         "max_steps",
					Label:       "Max Steps",
					InputType:   "number",
					Placeholder: "auto",
					Help:        "Optional per-episode step cap.",
				},
				{
					Key:          "episodes",
					Label:        "Episodes",
					InputType:    "number",
					DefaultValue: "0",
					Help:         "0 runs forever; >0 sets total episodes.",
				},
				{
					Key:         "map_dir",
					Label:       "Map Directory",
					InputType:   "text",
					Placeholder: "maps/gridworld",
					Help:        "Optional override path for map files.",
				},
			},
		},
	},
	// Future games can be added here with their own argument parsing
}
