package notify

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"unicode"

	"github.com/jstrah00/ccmcli/internal/config"
	"github.com/jstrah00/ccmcli/internal/tmux"
)

func Title(r Request) string {
	if r.Session == "" {
		return r.Folder
	}
	t := fmt.Sprintf("%s:%d", r.Session, r.Window)
	if r.Folder != "" && r.Folder != r.Session {
		t += "/" + r.Folder
	}
	return t
}

func Subtitle(k Kind) string {
	if k == Input {
		return "Needs your input"
	}
	return "Finished"
}

func Sound(k Kind) string {
	if k == Input {
		return "Sosumi"
	}
	return "Glass"
}

func body(r Request) string {
	if r.Body == "" {
		return Subtitle(r.Kind)
	}
	return r.Body
}

var terminals = map[string]string{
	"ghostty":        "com.mitchellh.ghostty",
	"iTerm.app":      "com.googlecode.iterm2",
	"Apple_Terminal": "com.apple.Terminal",
	"WezTerm":        "com.github.wez.wezterm",
	"kitty":          "net.kovidgoyal.kitty",
}

// Bundle es la app que el click trae al frente. Dentro de tmux TERM_PROGRAM
// suele ser "tmux"; __CFBundleIdentifier lo hereda el server de tmux de la
// terminal que lo arrancó.
func Bundle(cfg string, env func(string) string) string {
	if cfg != "" {
		return cfg
	}
	if b := env("__CFBundleIdentifier"); b != "" {
		return b
	}
	return terminals[env("TERM_PROGRAM")]
}

// El pane id va dentro de un comando de shell que corre terminal-notifier.
var paneID = regexp.MustCompile(`^%[0-9]+$`)

func MacArgs(r Request, bundle, tmuxBin string) []string {
	msg := body(r)
	// terminal-notifier rechaza un -message que arranca con [ ( { - o comillas.
	if first := []rune(msg)[0]; !unicode.IsLetter(first) && !unicode.IsDigit(first) {
		msg = `\` + msg
	}
	group := r.SessionID
	if group == "" {
		group = r.PaneID
	}
	// Sin -sender: con -sender el click solo activa esa app e ignora -execute.
	a := []string{"-title", Title(r), "-subtitle", Subtitle(r.Kind), "-message", msg, "-group", "claude-" + group}
	if bundle != "" {
		a = append(a, "-activate", bundle)
	}
	if tmuxBin != "" && paneID.MatchString(r.PaneID) {
		a = append(a, "-execute", fmt.Sprintf("'%s' switch-client -t '%s'", tmuxBin, r.PaneID))
	}
	return a
}

// OsaArgs pasa los textos por argv para no interpolarlos en el AppleScript.
func OsaArgs(r Request) []string {
	return []string{
		"-e", "on run argv",
		"-e", "display notification (item 3 of argv) with title (item 1 of argv) subtitle (item 2 of argv)",
		"-e", "end run",
		Title(r), Subtitle(r.Kind), body(r),
	}
}

func LinuxArgs(r Request) []string {
	return []string{"--app-name=ccm", "--action=go=Go to session", "--wait", Title(r) + " · " + Subtitle(r.Kind), body(r)}
}

// Find busca name en PATH y después en extra: los hooks y terminal-notifier
// corren con un PATH mínimo que no siempre incluye Homebrew.
func Find(name string, extra ...string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, p := range extra {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return ""
}

func brew(name string) []string {
	return []string{"/opt/homebrew/bin/" + name, "/usr/local/bin/" + name}
}

// Send manda el aviso y, en Linux, espera el click para saltar al pane.
func Send(r Request, cfg config.Notify) error {
	tmuxBin, _ := tmux.Find()
	switch runtime.GOOS {
	case "darwin":
		var sound *exec.Cmd
		if cfg.Sound {
			sound = exec.Command("/usr/bin/afplay", "-v", "0.5", "/System/Library/Sounds/"+Sound(r.Kind)+".aiff")
			_ = sound.Start()
		}
		var err error
		if cfg.Banner {
			err = macBanner(r, Bundle(cfg.TerminalBundle, os.Getenv), tmuxBin)
		}
		if sound != nil && sound.Process != nil {
			_ = sound.Wait()
		}
		return err
	case "linux":
		if !cfg.Banner {
			return nil
		}
		ns := Find("notify-send")
		if ns == "" {
			return fmt.Errorf("notify-send not found")
		}
		out, err := exec.Command(ns, LinuxArgs(r)...).Output()
		if err != nil {
			return err
		}
		if strings.TrimSpace(string(out)) == "go" && tmuxBin != "" && paneID.MatchString(r.PaneID) {
			return exec.Command(tmuxBin, "switch-client", "-t", r.PaneID).Run()
		}
	}
	return nil
}

func macBanner(r Request, bundle, tmuxBin string) error {
	if tn := Find("terminal-notifier", brew("terminal-notifier")...); tn != "" {
		if err := exec.Command(tn, MacArgs(r, bundle, tmuxBin)...).Run(); err == nil {
			return nil
		}
	}
	// Sin terminal-notifier (o sin permiso) igual sale el banner, pero el click
	// no lleva al pane.
	return exec.Command("/usr/bin/osascript", OsaArgs(r)...).Run()
}
