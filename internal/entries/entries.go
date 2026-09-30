// Package entries arma la lista de sesiones de Claude Code cruzando tmux con
// los archivos de estado y los títulos.
package entries

import (
	"path/filepath"
	"regexp"
	"time"

	"ccm/internal/order"
	"ccm/internal/state"
	"ccm/internal/tmux"
	"ccm/internal/transcript"
)

type Entry struct {
	Key        string
	Pane       tmux.PaneInfo
	State      state.State
	Since      time.Time
	Folder     string // basename del cwd
	Title      string // título de la fila
	Detail     string // último ai-title, para la zona de detalle
	Num        int    // 1..N según el orden; 0 hasta Arrange
	LastPrompt string // último prompt del usuario
	Alias      string // nombre puesto desde la TUI; reemplaza la ubicación
}

type TitleSource func(sessionID, path string) transcript.Titles

// El binario nativo de Claude Code se llama por su versión, así que
// pane_current_command llega como "2.1.280" y no como "claude".
var versionName = regexp.MustCompile(`^\d+\.\d+\.\d+`)

func isClaude(cmd string) bool {
	return cmd == "claude" || versionName.MatchString(cmd)
}

func Collect(panes []tmux.PaneInfo, states map[string]state.Pane, titles TitleSource) []Entry {
	var out []Entry
	for _, p := range panes {
		if !isClaude(p.Command) {
			continue
		}
		// La clave usa pane_current_path (no el cwd del estado) para que no
		// cambie cuando el pane pasa de unknown a tener estado.
		e := Entry{
			Key:    order.Key(p.Session, p.Window, p.Pane, p.Path),
			Pane:   p,
			State:  state.Unknown,
			Folder: filepath.Base(p.Path),
		}
		if st, ok := states[p.PaneID]; ok {
			e.State, e.Since, e.LastPrompt = st.State, st.Since, st.LastPrompt
			if st.Cwd != "" {
				e.Folder = filepath.Base(st.Cwd)
			}
			t := titles(st.SessionID, st.TranscriptPath)
			e.Title, e.Detail = t.Row(), t.LastAITitle
		}
		out = append(out, e)
	}
	return out
}

func Keys(es []Entry) []string {
	ks := make([]string, len(es))
	for i, e := range es {
		ks[i] = e.Key
	}
	return ks
}

func Arrange(all []Entry, keys []string) []Entry {
	byKey := make(map[string]Entry, len(all))
	for _, e := range all {
		byKey[e.Key] = e
	}
	out := make([]Entry, 0, len(keys))
	for _, k := range keys {
		e, ok := byKey[k]
		if !ok {
			continue
		}
		e.Num = len(out) + 1
		out = append(out, e)
	}
	return out
}

func Purge(s state.Store, panes []tmux.PaneInfo, states map[string]state.Pane) {
	alive := make(map[string]bool, len(panes))
	for _, p := range panes {
		alive[p.PaneID] = true
	}
	for id := range states {
		if !alive[id] {
			_ = s.Delete(id)
		}
	}
}
