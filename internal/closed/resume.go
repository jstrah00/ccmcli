package closed

import (
	"errors"
	"fmt"
	"os"
	"regexp"

	"ccm/internal/tmux"
)

type Tmux interface {
	HasSession(name string) bool
	CurrentSession() (string, error)
	NewWindow(session, cwd string) (tmux.PaneInfo, error)
	SendLine(paneID, line string) error
	Switch(p tmux.PaneInfo) error
}

var ErrBadSession = errors.New("session id inválido")

// El id se tipea en un shell: closed.json es editable a mano.
var safeID = regexp.MustCompile(`^[0-9A-Za-z_-]+$`)

// Resume abre una window con un shell y le tipea el comando, en vez de correr
// claude como comando de la window: así al salir de Claude queda el shell,
// como en una sesión abierta a mano, y se usa el PATH interactivo.
func Resume(c Session, t Tmux) (tmux.PaneInfo, error) {
	if !safeID.MatchString(c.SessionID) {
		return tmux.PaneInfo{}, ErrBadSession
	}
	if fi, err := os.Stat(c.Cwd); err != nil || !fi.IsDir() {
		return tmux.PaneInfo{}, fmt.Errorf("la carpeta ya no existe: %s", c.Cwd)
	}
	session := c.TmuxSession
	if session == "" || !t.HasSession(session) {
		cur, err := t.CurrentSession()
		if err != nil {
			return tmux.PaneInfo{}, err
		}
		session = cur
	}
	p, err := t.NewWindow(session, c.Cwd)
	if err != nil {
		return tmux.PaneInfo{}, err
	}
	if err := t.SendLine(p.PaneID, "claude --resume "+c.SessionID); err != nil {
		return tmux.PaneInfo{}, err
	}
	return p, t.Switch(p)
}
