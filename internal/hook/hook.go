// Package hook traduce eventos de Claude Code (y de tmux) a cambios de estado.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"ccm/internal/state"
	"ccm/internal/tmux"
)

// Input son los campos que usamos del JSON que Claude Code manda por stdin.
type Input struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Prompt         string `json:"prompt"`
}

type Deps struct {
	Store      state.Store
	PaneID     string
	Now        func() time.Time
	PaneActive func(paneID string) (bool, error)
}

var ErrUnknownEvent = errors.New("evento desconocido")

func Run(event string, stdin io.Reader, d Deps) error {
	if d.PaneID == "" {
		return nil // claude fuera de tmux: no hay a qué pane asociarlo
	}
	var in Input
	if b, err := io.ReadAll(stdin); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &in) // stdin roto: igual registramos la transición
	}

	var target state.State
	switch event {
	case "end":
		return d.Store.Delete(d.PaneID)
	case "start":
		target = state.Idle
	case "prompt":
		if injected(in.Prompt) {
			return nil
		}
		target = state.Working
	case "ask", "permission":
		target = state.Waiting
	case "tool-done":
		target = state.Working
	case "stop":
		target = state.Done
		if active, err := d.PaneActive(d.PaneID); err == nil && active {
			target = state.Idle // terminó mientras lo mirabas
		}
	default:
		return fmt.Errorf("%w: %q", ErrUnknownEvent, event)
	}

	p, err := d.Store.Load(d.PaneID)
	if err != nil {
		p = state.Pane{PaneID: d.PaneID}
	}
	if in.SessionID != "" {
		p.SessionID = in.SessionID
	}
	if in.Cwd != "" {
		p.Cwd = in.Cwd
	}
	if in.TranscriptPath != "" {
		p.TranscriptPath = in.TranscriptPath
	}
	return d.Store.Save(state.Apply(p, target, d.Now()))
}

// UserPromptSubmit también dispara cuando el harness inyecta un aviso como si
// fuera un prompt tuyo. Misma regla que notify.sh.
func injected(prompt string) bool {
	return strings.HasPrefix(prompt, "<task-notification>") ||
		strings.HasPrefix(prompt, "<system-reminder>")
}

// Seen pasa a idle los panes en done de toda window que estás mirando.
func Seen(c tmux.Client, s state.Store, now time.Time) error {
	panes, err := c.ListPanes()
	if err != nil {
		return err
	}
	for _, p := range panes {
		if !p.WindowActive || !p.SessionAttached {
			continue
		}
		st, err := s.Load(p.PaneID)
		if err != nil || st.State != state.Done {
			continue
		}
		if err := s.Save(state.Apply(st, state.Idle, now)); err != nil {
			return err
		}
	}
	return nil
}
