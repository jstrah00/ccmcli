// Package closed lleva el registro de las sesiones de Claude Code cerradas,
// para poder retomarlas desde la TUI.
package closed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/jstrah00/ccmcli/internal/fsutil"
	"github.com/jstrah00/ccmcli/internal/state"
)

const Max = 50

type Session struct {
	SessionID      string    `json:"session_id"`
	Cwd            string    `json:"cwd"`
	TranscriptPath string    `json:"transcript_path"`
	TmuxSession    string    `json:"tmux_session,omitempty"`
	Alias          string    `json:"alias,omitempty"`
	Status         string    `json:"status,omitempty"`
	LastPrompt     string    `json:"last_prompt"`
	ClosedAt       time.Time `json:"closed_at"`
}

func FromPane(p state.Pane, now time.Time) Session {
	return Session{
		SessionID:      p.SessionID,
		Cwd:            p.Cwd,
		TranscriptPath: p.TranscriptPath,
		TmuxSession:    p.TmuxSession,
		LastPrompt:     p.LastPrompt,
		ClosedAt:       now.UTC(),
	}
}

type Store struct{ Path string }

// List devuelve las cerradas, la más reciente primero. Un archivo corrupto o
// inexistente es una lista vacía.
func (s Store) List() []Session {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		return nil
	}
	var out []Session
	if json.Unmarshal(b, &out) != nil {
		return nil
	}
	return out
}

// Add ignora las sesiones sin id o sin ningún prompt: no hay nada que retomar.
func (s Store) Add(c Session) error {
	if c.SessionID == "" || c.LastPrompt == "" {
		return nil
	}
	return s.update(func(l []Session) []Session {
		l = slices.DeleteFunc(l, func(o Session) bool { return o.SessionID == c.SessionID })
		l = slices.Insert(l, 0, c)
		if len(l) > Max {
			l = l[:Max]
		}
		return l
	})
}

func (s Store) Take(sessionID string) (Session, bool, error) {
	var (
		found Session
		ok    bool
	)
	if sessionID == "" {
		return found, false, nil
	}
	err := s.update(func(l []Session) []Session {
		i := slices.IndexFunc(l, func(o Session) bool { return o.SessionID == sessionID })
		if i < 0 {
			return nil
		}
		found, ok = l[i], true
		return slices.Delete(l, i, i+1)
	})
	return found, ok, err
}

// update serializa el leer-modificar-escribir con un flock: al matar una
// sesión de tmux, todos sus claude corren `hook end` a la vez. fn devuelve
// nil para no escribir.
func (s Store) update(fn func([]Session) []Session) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	lock, err := os.OpenFile(s.Path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)

	next := fn(s.List())
	if next == nil {
		return nil
	}
	b, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.Path, b)
}
