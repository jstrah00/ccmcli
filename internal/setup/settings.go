// Package setup instala ccm: hooks de Claude Code y bloque de tmux.
package setup

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Event struct {
	Name    string // evento de Claude Code
	Matcher string
	Arg     string // ccm hook <Arg>
}

var Events = []Event{
	{"SessionStart", "", "start"},
	{"UserPromptSubmit", "", "prompt"},
	{"PreToolUse", "AskUserQuestion", "ask"},
	{"PostToolUse", "", "tool-done"},
	{"Notification", "permission_prompt", "permission"},
	{"Notification", "idle_prompt", "idle"},
	{"Stop", "", "stop"},
	{"SessionEnd", "", "end"},
}

func SettingsPath() string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "settings.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func ShellQuote(s string) string {
	if !strings.ContainsAny(s, " \t'\"$`\\") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func Command(bin, arg string) string { return ShellQuote(bin) + " hook " + arg }

func isCCM(cmd, arg string) bool {
	return regexp.MustCompile(`(^|[\s/'"])ccm['"]?\s+hook\s+` + regexp.QuoteMeta(arg) + `\s*$`).MatchString(cmd)
}

// MergeHooks deja un hook de ccm por evento con el binario bin. Saca los de
// ccm con otra ruta y no toca el resto. Si no hay cambios devuelve settings
// tal cual, para no reformatear el archivo del usuario.
func MergeHooks(settings []byte, bin string) ([]byte, bool, error) {
	if strings.TrimSpace(string(settings)) == "" {
		settings = []byte("{}")
	}
	v, err := parse(settings)
	if err != nil {
		return nil, false, err
	}
	root, ok := v.(obj)
	if !ok {
		return nil, false, errors.New("settings is not a JSON object")
	}
	before, err := encode(root)
	if err != nil {
		return nil, false, err
	}
	hv, ok := root.get("hooks")
	if !ok {
		hv = obj{}
	}
	hooks, ok := hv.(obj)
	if !ok {
		return nil, false, errors.New(`"hooks" is not a JSON object`)
	}
	for _, ev := range Events {
		var groups []any
		if g, ok := hooks.get(ev.Name); ok {
			if groups, ok = g.([]any); !ok {
				return nil, false, errors.New(`"hooks.` + ev.Name + `" is not a list`)
			}
		}
		hooks.set(ev.Name, mergeEvent(groups, ev, Command(bin, ev.Arg)))
	}
	root.set("hooks", hooks)
	after, err := encode(root)
	if err != nil {
		return nil, false, err
	}
	if string(after) == string(before) {
		return settings, false, nil
	}
	return after, true, nil
}

func mergeEvent(groups []any, ev Event, cmd string) []any {
	found := false
	out := []any{}
	for _, g := range groups {
		grp, ok := g.(obj)
		if !ok {
			out = append(out, g)
			continue
		}
		hv, _ := grp.get("hooks")
		hs, ok := hv.([]any)
		if !ok {
			out = append(out, g)
			continue
		}
		matcher, _ := grp.get("matcher")
		m, _ := matcher.(string)
		kept := []any{}
		removed := false
		for _, h := range hs {
			ho, _ := h.(obj)
			c, _ := ho.get("command")
			cs, _ := c.(string)
			switch {
			case cs == cmd && m == ev.Matcher && !found:
				found = true
				kept = append(kept, h)
			case isCCM(cs, ev.Arg):
				removed = true
			default:
				kept = append(kept, h)
			}
		}
		if len(kept) == 0 && removed {
			continue
		}
		grp.set("hooks", kept)
		out = append(out, grp)
	}
	if !found {
		g := obj{}
		if ev.Matcher != "" {
			g = append(g, kv{"matcher", ev.Matcher})
		}
		g = append(g, kv{"hooks", []any{obj{{"type", "command"}, {"command", cmd}, {"timeout", 5}}}})
		out = append(out, g)
	}
	return out
}
