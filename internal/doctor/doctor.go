// Package doctor revisa que ccm esté bien instalado.
package doctor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jstrah00/ccmcli/internal/setup"
)

type Status int

const (
	OK Status = iota
	Warn
	Fail
)

type Check struct {
	Name   string
	Status Status
	Fix    string
}

type Env struct {
	Settings     []byte
	SettingsPath string
	TmuxConf     string
	TmuxConfPath string
	Bin          string
	TmuxVersion  string // salida de tmux -V; vacío si no hay tmux
	GOOS         string
	Bundle       string
	ConfigErr    error
	Look         func(name string) string // ruta del binario o ""
}

func Run(e Env) []Check {
	cs := []Check{tmuxVersion(e.TmuxVersion), hooks(e), tmuxConf(e), notifier(e)}
	if e.GOOS == "darwin" {
		c := Check{Name: "terminal app: " + e.Bundle}
		if e.Bundle == "" {
			c = Check{Name: "terminal app", Status: Warn,
				Fix: `can't tell which terminal to bring to the front on click; set "terminal_bundle" in ~/.config/ccm/config.json`}
		}
		cs = append(cs, c)
	}
	cs = append(cs, otherNotifiers(e))
	if e.ConfigErr != nil {
		cs = append(cs, Check{Name: "config file", Status: Warn, Fix: e.ConfigErr.Error() + " (using defaults)"})
	}
	return cs
}

var versionRe = regexp.MustCompile(`(\d+)\.(\d+)`)

// display-popup llegó en tmux 3.2.
func tmuxVersion(v string) Check {
	c := Check{Name: "tmux ≥ 3.2"}
	m := versionRe.FindStringSubmatch(v)
	if m == nil {
		return Check{Name: c.Name, Status: Fail, Fix: "install tmux 3.2 or newer"}
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	if major < 3 || major == 3 && minor < 2 {
		return Check{Name: c.Name, Status: Fail, Fix: "found " + v + ": ccm needs display-popup (tmux 3.2+)"}
	}
	c.Name += " (" + v + ")"
	return c
}

type settings struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
}

func hooks(e Env) Check {
	name := "Claude Code hooks"
	var s settings
	if err := json.Unmarshal(e.Settings, &s); err != nil {
		return Check{Name: name, Status: Fail, Fix: fmt.Sprintf("can't read %s: %v", e.SettingsPath, err)}
	}
	var missing []string
	for _, ev := range setup.Events {
		want := setup.Command(e.Bin, ev.Arg)
		found := false
		for _, g := range s.Hooks[ev.Name] {
			for _, h := range g.Hooks {
				if h.Command == want && g.Matcher == ev.Matcher {
					found = true
				}
			}
		}
		if !found {
			missing = append(missing, ev.Arg)
		}
	}
	if len(missing) > 0 {
		return Check{Name: name, Status: Fail,
			Fix: fmt.Sprintf("missing or pointing to another binary (%s): run `ccm setup`", strings.Join(missing, ", "))}
	}
	return Check{Name: name}
}

func tmuxConf(e Env) Check {
	name := "tmux config"
	if !strings.Contains(e.TmuxConf, setup.BlockStart) || !strings.Contains(e.TmuxConf, `"`+setup.ShellQuote(e.Bin)+`"`) {
		return Check{Name: name, Status: Fail, Fix: "ccm block missing or pointing to another binary in " + e.TmuxConfPath + ": run `ccm setup`"}
	}
	if s := setup.StrayLines(e.TmuxConf); len(s) > 0 {
		return Check{Name: name, Status: Warn, Fix: fmt.Sprintf("ccm lines outside the ccm block in %s (lines %v): remove them", e.TmuxConfPath, s)}
	}
	return Check{Name: name}
}

func notifier(e Env) Check {
	switch e.GOOS {
	case "darwin":
		if p := e.Look("terminal-notifier"); p != "" {
			return Check{Name: "notifications: " + p}
		}
		return Check{Name: "notifications", Status: Warn,
			Fix: "terminal-notifier not found, falling back to osascript (clicking won't jump to the pane): brew install terminal-notifier"}
	case "linux":
		if p := e.Look("notify-send"); p != "" {
			return Check{Name: "notifications: " + p}
		}
		return Check{Name: "notifications", Status: Warn, Fix: "notify-send not found: install libnotify (e.g. libnotify-bin)"}
	}
	return Check{Name: "notifications", Status: Warn, Fix: "not supported on " + e.GOOS}
}

var notifierRe = regexp.MustCompile(`(?i)notif|afplay|osascript|\bsay\b`)

func otherNotifiers(e Env) Check {
	name := "duplicate notifications"
	var s settings
	_ = json.Unmarshal(e.Settings, &s)
	var found []string
	for _, ev := range []string{"Stop", "Notification"} {
		for _, g := range s.Hooks[ev] {
			for _, h := range g.Hooks {
				if !strings.Contains(h.Command, " hook ") && notifierRe.MatchString(h.Command) {
					found = append(found, h.Command)
				}
			}
		}
	}
	if len(found) > 0 {
		return Check{Name: name, Status: Warn,
			Fix: "other hooks may notify too, remove them to avoid double alerts: " + strings.Join(found, "; ")}
	}
	return Check{Name: "no " + name}
}

func Print(cs []Check) (string, bool) {
	var b strings.Builder
	ok := true
	for _, c := range cs {
		mark := map[Status]string{OK: "✔", Warn: "!", Fail: "✘"}[c.Status]
		if c.Status == Fail {
			ok = false
		}
		fmt.Fprintf(&b, "%s %s", mark, c.Name)
		if c.Fix != "" {
			fmt.Fprintf(&b, ": %s", c.Fix)
		}
		b.WriteByte('\n')
	}
	return b.String(), ok
}
