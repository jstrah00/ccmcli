// Package proc encuentra el pty-host de Claude Code entre los ancestros del
// proceso actual.
//
// Cuando una sesión pasa a correr en el daemon de Claude Code (sessionKind
// "bg", p. ej. tras EnterWorktree/ExitWorktree), los hooks los ejecuta un
// proceso `claude --bg-pty-host` que no hereda TMUX_PANE. Sus args son lo único
// que lo conecta con la sesión que antes corría en el pane.
package proc

import (
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// BgHost identifica un pty-host: SessionID es su --session-id (fijo mientras
// viva el host) y Resumed el session id del transcript de --resume.
type BgHost struct {
	SessionID string
	Resumed   string
}

// Lookup devuelve el ppid y los args de un proceso.
type Lookup func(pid int) (ppid int, args []string, err error)

// maxDepth: hook -> sh -> pty-host son 2 saltos; el margen cubre wrappers.
const maxDepth = 8

func FindBgHost(pid int, look Lookup) (BgHost, bool) {
	for i := 0; i < maxDepth && pid > 1; i++ {
		ppid, args, err := look(pid)
		if err != nil {
			return BgHost{}, false
		}
		if h, ok := ParseBgHost(args); ok {
			return h, true
		}
		pid = ppid
	}
	return BgHost{}, false
}

func ParseBgHost(args []string) (BgHost, bool) {
	var h BgHost
	isHost := false
	for i, a := range args {
		switch a {
		case "--bg-pty-host":
			isHost = true
		case "--session-id":
			if i+1 < len(args) {
				h.SessionID = args[i+1]
			}
		case "--resume":
			if i+1 < len(args) {
				h.Resumed = strings.TrimSuffix(filepath.Base(args[i+1]), ".jsonl")
			}
		}
	}
	return h, isHost && h.SessionID != ""
}

// PS implementa Lookup con ps. Los args se separan por espacios: alcanza para
// flags y UUIDs, que es lo único que se lee.
func PS(pid int) (int, []string, error) {
	p := strconv.Itoa(pid)
	out, err := exec.Command("ps", "-o", "ppid=", "-p", p).Output()
	if err != nil {
		return 0, nil, err
	}
	ppid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, nil, err
	}
	out, err = exec.Command("ps", "-o", "args=", "-p", p).Output()
	if err != nil {
		return 0, nil, err
	}
	return ppid, strings.Fields(string(out)), nil
}
