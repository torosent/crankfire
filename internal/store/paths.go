// internal/store/paths.go
package store

import (
	"fmt"
	"os"
	"path/filepath"
)

func ResolveDataDir(explicit string) (string, error) {
	if explicit != "" { return explicit, nil }
	if env := os.Getenv("CRANKFIRE_DATA_DIR"); env != "" { return env, nil }
	home, err := os.UserHomeDir()
	if err != nil { return "", fmt.Errorf("resolve data dir: %w", err) }
	return filepath.Join(home, ".crankfire"), nil
}

func sessionsDir(dataDir string) string { return filepath.Join(dataDir, "sessions") }
func runsDir(dataDir string) string     { return filepath.Join(dataDir, "runs") }
func sessionPath(dataDir, id string) string {
	return filepath.Join(sessionsDir(dataDir), id+".yaml")
}
func runDir(dataDir, sessionID, started string) string {
	return filepath.Join(runsDir(dataDir), sessionID, started)
}
