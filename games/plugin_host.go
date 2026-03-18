package games

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/JonKirkpatrick/bbs/games/pluginapi"
)

const (
	gamePluginsEnabledEnv   = "BBS_ENABLE_GAME_PLUGINS"
	gamePluginsDirectoryEnv = "BBS_GAME_PLUGIN_DIR"
	defaultGamePluginsDir   = "plugins/games"
	pluginRefreshInterval   = 2 * time.Second
)

type pluginRegistryCacheState struct {
	mu          sync.Mutex
	refreshedAt time.Time
	directory   string
	entries     map[string]gameRegistration
}

var pluginRegistryCache pluginRegistryCacheState

func allRegistrations() map[string]gameRegistration {
	registrations := make(map[string]gameRegistration, len(builtinRegistry))
	for name, registration := range builtinRegistry {
		registrations[name] = registration
	}

	for name, registration := range dynamicPluginRegistrations() {
		if _, exists := registrations[name]; exists {
			continue
		}
		registrations[name] = registration
	}

	return registrations
}

func pluginsEnabled() bool {
	raw := strings.TrimSpace(os.Getenv(gamePluginsEnabledEnv))
	if raw == "" {
		return false
	}
	enabled, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return enabled
}

func pluginsDirectory() string {
	dir := strings.TrimSpace(os.Getenv(gamePluginsDirectoryEnv))
	if dir == "" {
		dir = defaultGamePluginsDir
	}
	return filepath.Clean(dir)
}

func dynamicPluginRegistrations() map[string]gameRegistration {
	if !pluginsEnabled() {
		return nil
	}

	directory := pluginsDirectory()
	now := time.Now()

	pluginRegistryCache.mu.Lock()
	defer pluginRegistryCache.mu.Unlock()

	if pluginRegistryCache.entries != nil &&
		pluginRegistryCache.directory == directory &&
		now.Sub(pluginRegistryCache.refreshedAt) < pluginRefreshInterval {
		return cloneGameRegistrations(pluginRegistryCache.entries)
	}

	scanned := scanPluginDirectory(directory)
	pluginRegistryCache.entries = scanned
	pluginRegistryCache.directory = directory
	pluginRegistryCache.refreshedAt = now
	return cloneGameRegistrations(scanned)
}

func cloneGameRegistrations(source map[string]gameRegistration) map[string]gameRegistration {
	if len(source) == 0 {
		return nil
	}
	copyMap := make(map[string]gameRegistration, len(source))
	for name, entry := range source {
		copyMap[name] = entry
	}
	return copyMap
}

func scanPluginDirectory(directory string) map[string]gameRegistration {
	files, err := filepath.Glob(filepath.Join(directory, "*.json"))
	if err != nil || len(files) == 0 {
		return nil
	}

	sort.Strings(files)
	entries := make(map[string]gameRegistration)

	for _, manifestPath := range files {
		registration, name, ok := registrationFromManifest(directory, manifestPath)
		if !ok {
			continue
		}
		entries[name] = registration
	}

	if len(entries) == 0 {
		return nil
	}

	return entries
}

func registrationFromManifest(directory, manifestPath string) (gameRegistration, string, bool) {
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		fmt.Printf("[game-plugin] failed reading manifest %s: %v\n", manifestPath, err)
		return gameRegistration{}, "", false
	}

	var manifest pluginapi.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		fmt.Printf("[game-plugin] failed decoding manifest %s: %v\n", manifestPath, err)
		return gameRegistration{}, "", false
	}

	if manifest.ProtocolVersion == 0 {
		manifest.ProtocolVersion = pluginapi.ProtocolVersion
	}
	if manifest.ProtocolVersion != pluginapi.ProtocolVersion {
		fmt.Printf("[game-plugin] skipping %s: protocol_version=%d (expected %d)\n", manifestPath, manifest.ProtocolVersion, pluginapi.ProtocolVersion)
		return gameRegistration{}, "", false
	}

	name := strings.ToLower(strings.TrimSpace(manifest.Name))
	if name == "" {
		fmt.Printf("[game-plugin] skipping %s: missing name\n", manifestPath)
		return gameRegistration{}, "", false
	}

	execPath, err := resolvePluginExecutable(directory, manifestPath, manifest.Executable)
	if err != nil {
		fmt.Printf("[game-plugin] skipping %s: %v\n", manifestPath, err)
		return gameRegistration{}, "", false
	}

	args := make([]GameArgSpec, 0, len(manifest.Args))
	for _, arg := range manifest.Args {
		args = append(args, GameArgSpec{
			Key:          arg.Key,
			Label:        arg.Label,
			InputType:    arg.InputType,
			Placeholder:  arg.Placeholder,
			DefaultValue: arg.DefaultValue,
			Required:     arg.Required,
			Help:         arg.Help,
		})
	}

	displayName := strings.TrimSpace(manifest.DisplayName)
	if displayName == "" {
		displayName = name
	}

	capturedName := name
	capturedExecutable := execPath
	registration := gameRegistration{
		Factory: func(args []string) (GameInstance, error) {
			return launchPluginGame(capturedName, capturedExecutable, args)
		},
		Catalog: GameCatalogEntry{
			Name:              name,
			DisplayName:       displayName,
			Args:              args,
			SupportsMoveClock: manifest.SupportsMoveClock,
			SupportsHandicap:  manifest.SupportsHandicap,
		},
	}

	return registration, capturedName, true
}

func resolvePluginExecutable(directory, manifestPath, executable string) (string, error) {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return "", fmt.Errorf("manifest %s is missing executable", manifestPath)
	}

	tryPaths := make([]string, 0, 3)
	if filepath.IsAbs(executable) {
		tryPaths = append(tryPaths, executable)
	} else {
		tryPaths = append(tryPaths,
			filepath.Join(filepath.Dir(manifestPath), executable),
			filepath.Join(directory, executable),
			executable,
		)
	}

	for _, candidate := range tryPaths {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Clean(candidate), nil
		}
	}

	return "", fmt.Errorf("executable %q was not found", executable)
}
