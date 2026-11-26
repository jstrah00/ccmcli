// Package tmux habla con el server de tmux. Todo pasa por la interfaz Client
// para poder testear el resto con Fake.
package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type PaneInfo struct {
	PaneID          string
	Session         string
	Window          int
	Pane            int
	Command         string
	Path            string
	WindowActive    bool
	SessionAttached bool
	WindowName      string
}

type Client interface {
	ListPanes() ([]PaneInfo, error)
	PaneActive(paneID string) (bool, error)
	Switch(p PaneInfo) error
}

// Tab como separador: los nombres de sesión pueden tener espacios.
const listFormat = "#{pane_id}\t#{session_name}\t#{window_index}\t#{pane_index}\t" +
	"#{pane_current_command}\t#{pane_current_path}\t#{window_active}\t#{session_attached}\t#{window_name}"

func ParsePanes(out string) ([]PaneInfo, error) {
	var ps []PaneInfo
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 9 {
			return nil, fmt.Errorf("unexpected list-panes line: %q", line)
		}
		w, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, err
		}
		p, err := strconv.Atoi(f[3])
		if err != nil {
			return nil, err
		}
		ps = append(ps, PaneInfo{
			PaneID: f[0], Session: f[1], Window: w, Pane: p,
			Command: f[4], Path: f[5],
			WindowActive:    f[6] == "1",
			SessionAttached: f[7] != "" && f[7] != "0",
			WindowName:      f[8],
		})
	}
	return ps, nil
}

// Find busca el binario de tmux. En la Mac suele estar en /opt/homebrew/bin,
// que puede no estar en el PATH de un hook.
func Find() (string, error) {
	if p, err := exec.LookPath("tmux"); err == nil {
		return p, nil
	}
	for _, p := range []string{"/usr/bin/tmux", "/opt/homebrew/bin/tmux", "/usr/local/bin/tmux"} {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", errors.New("tmux not found")
}

type Exec struct {
	Bin    string
	Socket string
}

func (e Exec) run(args ...string) (string, error) {
	if e.Socket != "" {
		args = append([]string{"-L", e.Socket}, args...)
	}
	cmd := exec.Command(e.Bin, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

func (e Exec) ListPanes() ([]PaneInfo, error) {
	out, err := e.run("list-panes", "-a", "-F", listFormat)
	if err != nil {
		return nil, err
	}
	return ParsePanes(out)
}

// PaneActive: la window del pane es la activa de su sesión y la sesión tiene
// algún cliente attached, o sea, lo estás mirando.
func (e Exec) PaneActive(paneID string) (bool, error) {
	out, err := e.run("display-message", "-p", "-t", paneID, "#{window_active} #{session_attached}")
	if err != nil {
		return false, err
	}
	f := strings.Fields(out)
	return len(f) == 2 && f[0] == "1" && f[1] != "0", nil
}

func (e Exec) PaneLoc(paneID string) (PaneInfo, error) {
	out, err := e.run("display-message", "-p", "-t", paneID, listFormat)
	if err != nil {
		return PaneInfo{}, err
	}
	return one(out)
}

func one(out string) (PaneInfo, error) {
	ps, err := ParsePanes(out)
	if err != nil {
		return PaneInfo{}, err
	}
	if len(ps) != 1 {
		return PaneInfo{}, fmt.Errorf("expected one pane, got %d", len(ps))
	}
	return ps[0], nil
}

// CurrentPane: sin -t, tmux resuelve el pane activo del cliente que abrió el
// popup.
func (e Exec) CurrentPane() (string, error) {
	out, err := e.run("display-message", "-p", "#{pane_id}")
	return strings.TrimSpace(out), err
}

func (e Exec) HasSession(name string) bool {
	_, err := e.run("has-session", "-t", "="+name)
	return err == nil
}

func (e Exec) CurrentSession() (string, error) {
	out, err := e.run("display-message", "-p", "#{session_name}")
	return strings.TrimRight(out, "\n"), err
}

// "=sesión:" sin índice: tmux usa el próximo libre.
func newWindowArgs(session, cwd string) []string {
	return []string{"new-window", "-d", "-P", "-F", listFormat, "-t", "=" + session + ":", "-c", cwd}
}

func (e Exec) NewWindow(session, cwd string) (PaneInfo, error) {
	out, err := e.run(newWindowArgs(session, cwd)...)
	if err != nil {
		return PaneInfo{}, err
	}
	return one(out)
}

func (e Exec) SendLine(paneID, line string) error {
	if _, err := e.run("send-keys", "-t", paneID, "-l", line); err != nil {
		return err
	}
	_, err := e.run("send-keys", "-t", paneID, "Enter")
	return err
}

func switchArgs(p PaneInfo) [][]string {
	return [][]string{
		{"switch-client", "-t", "=" + p.Session},
		{"select-window", "-t", fmt.Sprintf("=%s:%d", p.Session, p.Window)},
		{"select-pane", "-t", p.PaneID},
	}
}

func (e Exec) Switch(p PaneInfo) error {
	for _, a := range switchArgs(p) {
		if _, err := e.run(a...); err != nil {
			return err
		}
	}
	return nil
}
