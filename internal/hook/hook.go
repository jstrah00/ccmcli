// Package hook traduce eventos de Claude Code (y de tmux) a cambios de estado.
package hook

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/jstrah00/ccmcli/internal/closed"
	"github.com/jstrah00/ccmcli/internal/notify"
	"github.com/jstrah00/ccmcli/internal/order"
	"github.com/jstrah00/ccmcli/internal/proc"
	"github.com/jstrah00/ccmcli/internal/state"
	"github.com/jstrah00/ccmcli/internal/tmux"
)

// Input son los campos que usamos del JSON que Claude Code manda por stdin.
type Input struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	Prompt         string `json:"prompt"`
	// Stop
	LastAssistantMessage string `json:"last_assistant_message"`
	// Notification
	Message string `json:"message"`
	// PreToolUse de AskUserQuestion
	ToolInput struct {
		Questions []struct {
			Question string `json:"question"`
		} `json:"questions"`
	} `json:"tool_input"`
}

type Deps struct {
	Store      state.Store
	PaneID     string
	Now        func() time.Time
	PaneActive func(paneID string) (bool, error)
	// BgHost se usa cuando falta PaneID: el hook corre en el daemon.
	BgHost func() (proc.BgHost, bool)
	// Registro de cerradas; con Path vacío no se usa. PaneLoc y Labels son
	// opcionales.
	Closed  closed.Store
	PaneLoc func(paneID string) (tmux.PaneInfo, error)
	Labels  func(key string) (alias, status string)
	// Notify recibe los avisos a mandar; nil desactiva los avisos.
	Notify  func(notify.Request)
	MinTurn time.Duration
	Socket  string // socket del server de tmux, para el click del aviso
}

var ErrUnknownEvent = errors.New("unknown event")

func Run(event string, stdin io.Reader, d Deps) error {
	var host string
	if d.PaneID == "" {
		d.PaneID, host = resolveBg(d)
		if d.PaneID == "" {
			return nil // claude fuera de tmux: no hay a qué pane asociarlo
		}
	}
	var in Input
	if b, err := io.ReadAll(stdin); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &in) // stdin roto: igual registramos la transición
	}

	var target state.State
	switch event {
	case "end":
		err := recordClosed(d)
		if derr := d.Store.Delete(d.PaneID); derr != nil {
			return derr
		}
		return err
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
		target = finished(d)
	case "idle":
		// idle_prompt: CC espera un prompt nuevo. Stop no dispara si
		// interrumpiste con Esc, así que esto destraba working/waiting.
		p, err := d.Store.Load(d.PaneID)
		if err != nil || (p.State != state.Working && p.State != state.Waiting) {
			return nil
		}
		target = finished(d)
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
	if host != "" {
		p.PtyHost = host
	}
	if event == "start" {
		if d.PaneLoc != nil {
			if l, err := d.PaneLoc(d.PaneID); err == nil {
				p.TmuxSession = l.Session
			}
		}
		// Una sesión retomada sale de las cerradas. Hereda el último prompt
		// para volver a la lista si se cierra sin escribirle nada.
		if d.Closed.Path != "" {
			if c, ok, _ := d.Closed.Take(in.SessionID); ok && p.LastPrompt == "" {
				p.LastPrompt = c.LastPrompt
			}
		}
	}
	if event == "prompt" && in.Prompt != "" {
		p.LastPrompt = oneLine(in.Prompt, maxPrompt)
	}
	if event == "prompt" {
		p.TurnStart = d.Now().UTC()
	}
	p = state.Apply(p, target, d.Now())
	if err := d.Store.Save(p); err != nil {
		return err
	}
	maybeNotify(d, event, in, p)
	return nil
}

// maxBody: runas del cuerpo del aviso; más no entra en un banner.
const maxBody = 110

func maybeNotify(d Deps, event string, in Input, p state.Pane) {
	if d.Notify == nil {
		return
	}
	r := notify.Request{PaneID: d.PaneID, SessionID: p.SessionID, Socket: d.Socket}
	switch event {
	case "permission", "ask":
		if active, err := d.PaneActive(d.PaneID); err == nil && active {
			return
		}
		r.Kind, r.Body = notify.Input, in.Message
		if event == "ask" && len(in.ToolInput.Questions) > 0 {
			r.Body = in.ToolInput.Questions[0].Question
		}
	case "stop":
		if p.State != state.Done {
			return
		}
		if !p.TurnStart.IsZero() && d.Now().Sub(p.TurnStart) < d.MinTurn {
			return
		}
		r.Kind, r.Body = notify.Finished, in.LastAssistantMessage
	default:
		return
	}
	r.Body = oneLine(r.Body, maxBody)
	if p.Cwd != "" {
		r.Folder = filepath.Base(p.Cwd)
	}
	if d.PaneLoc != nil {
		if l, err := d.PaneLoc(d.PaneID); err == nil {
			r.Session, r.Window = l.Session, l.Window
		}
	}
	d.Notify(r)
}

func recordClosed(d Deps) error {
	if d.Closed.Path == "" {
		return nil
	}
	p, err := d.Store.Load(d.PaneID)
	if err != nil {
		return nil // sin estado no hay sesión que registrar
	}
	c := closed.FromPane(p, d.Now())
	if d.PaneLoc != nil {
		if l, err := d.PaneLoc(d.PaneID); err == nil {
			c.TmuxSession = l.Session
			if d.Labels != nil {
				c.Alias, c.Status = d.Labels(order.Key(l.Session, l.Window, l.Pane, l.Path))
			}
		}
	}
	return d.Closed.Add(c)
}

// resolveBg busca el pane de una sesión que pasó al daemon: primero por el
// pty-host ya asociado y si no por la sesión que el host resumió, que es la
// que registraron los hooks cuando todavía corría en el pane.
func resolveBg(d Deps) (paneID, host string) {
	if d.BgHost == nil {
		return "", ""
	}
	h, ok := d.BgHost()
	if !ok {
		return "", ""
	}
	all, err := d.Store.All()
	if err != nil {
		return "", ""
	}
	for _, p := range all {
		if p.PtyHost == h.SessionID {
			return p.PaneID, h.SessionID
		}
	}
	if h.Resumed == "" {
		return "", ""
	}
	for _, p := range all {
		if p.SessionID == h.Resumed {
			return p.PaneID, h.SessionID
		}
	}
	return "", ""
}

// maxPrompt: runas del último prompt que se guardan; alcanza para una línea
// de detalle en cualquier popup.
const maxPrompt = 300

// oneLine colapsa saltos de línea, tabs y espacios repetidos (se muestra en
// una sola línea) y lo recorta a max runas.
func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > max {
		s = string(r[:max])
	}
	return s
}

// finished: done, o idle si terminó mientras lo mirabas.
func finished(d Deps) state.State {
	if active, err := d.PaneActive(d.PaneID); err == nil && active {
		return state.Idle
	}
	return state.Done
}

// UserPromptSubmit también dispara cuando el harness inyecta un aviso como si
// fuera un prompt tuyo.
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
