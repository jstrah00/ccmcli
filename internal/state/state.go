// Package state guarda el estado de cada sesión de Claude Code, un archivo
// JSON por pane de tmux.
package state

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jstrah00/ccmcli/internal/fsutil"
)

type State string

const (
	Idle    State = "idle"
	Working State = "working"
	Waiting State = "waiting"
	Done    State = "done"
	Unknown State = "unknown"
)

type Pane struct {
	PaneID         string    `json:"pane_id"`
	SessionID      string    `json:"session_id"`
	Cwd            string    `json:"cwd"`
	TranscriptPath string    `json:"transcript_path"`
	State          State     `json:"state"`
	Since          time.Time `json:"since"`
	LastPrompt     string    `json:"last_prompt,omitempty"`
	// PtyHost: --session-id del pty-host del daemon cuando la sesión corre en
	// background; sus hooks no traen TMUX_PANE y se resuelven por este campo.
	PtyHost string `json:"pty_host,omitempty"`
	// TmuxSession se anota en start para saber dónde retomar la sesión si el
	// pane muere sin pasar por el hook end.
	TmuxSession string `json:"tmux_session,omitempty"`
}

// Apply devuelve p con el estado st. Since solo se mueve si el estado cambia.
func Apply(p Pane, st State, now time.Time) Pane {
	if p.State != st {
		p.State = st
		p.Since = now.UTC()
	}
	return p
}

type Store struct{ Dir string }

// "%41" -> "41.json"
func fileName(paneID string) string { return strings.TrimPrefix(paneID, "%") + ".json" }

func (s Store) path(paneID string) string { return filepath.Join(s.Dir, fileName(paneID)) }

func (s Store) Load(paneID string) (Pane, error) {
	b, err := os.ReadFile(s.path(paneID))
	if err != nil {
		return Pane{}, err
	}
	var p Pane
	if err := json.Unmarshal(b, &p); err != nil {
		return Pane{}, err
	}
	return p, nil
}

func (s Store) Save(p Pane) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path(p.PaneID), b)
}

func (s Store) Delete(paneID string) error {
	err := os.Remove(s.path(paneID))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// All devuelve los estados válidos indexados por pane id. Archivos corruptos
// o temporales se ignoran: esos panes se verán como Unknown.
func (s Store) All() (map[string]Pane, error) {
	out := map[string]Pane{}
	ents, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Dir, e.Name()))
		if err != nil {
			continue
		}
		var p Pane
		if json.Unmarshal(b, &p) != nil || p.PaneID == "" {
			continue
		}
		out[p.PaneID] = p
	}
	return out, nil
}
