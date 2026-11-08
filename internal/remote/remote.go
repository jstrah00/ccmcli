// Package remote detecta qué sesiones de Claude Code tienen remote-control
// activo. Claude Code no lo expone a los hooks: lo deja en
// ~/.claude/sessions/<pid>.json como bridgeSessionId (null o ausente si está
// apagado).
package remote

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func Dir() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "sessions")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "sessions")
}

// Active devuelve los session ids con remote-control activo.
func Active(dir string) map[string]bool {
	out := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var s struct {
			SessionID       string  `json:"sessionId"`
			BridgeSessionID *string `json:"bridgeSessionId"`
		}
		if json.Unmarshal(b, &s) != nil || s.SessionID == "" {
			continue
		}
		if s.BridgeSessionID != nil && *s.BridgeSessionID != "" {
			out[s.SessionID] = true
		}
	}
	return out
}
