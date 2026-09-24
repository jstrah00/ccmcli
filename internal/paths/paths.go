// Package paths resuelve dónde guarda ccm su estado, respetando XDG.
package paths

import (
	"os"
	"path/filepath"
)

func base(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

func CacheDir() string  { return filepath.Join(base("XDG_CACHE_HOME", ".cache"), "ccm") }
func StateDir() string  { return filepath.Join(base("XDG_STATE_HOME", filepath.Join(".local", "state")), "ccm") }
func PanesDir() string  { return filepath.Join(CacheDir(), "panes") }
func TitlesDir() string { return filepath.Join(CacheDir(), "titles") }
func LogPath() string   { return filepath.Join(CacheDir(), "log") }
func OrderPath() string { return filepath.Join(StateDir(), "order.json") }
