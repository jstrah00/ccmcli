package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/jstrah00/ccmcli/internal/setup"
)

const bin = "/usr/local/bin/ccm"

func good(t *testing.T) Env {
	s, _, err := setup.MergeHooks([]byte(`{}`), bin)
	if err != nil {
		t.Fatal(err)
	}
	return Env{
		Settings: s, TmuxConf: setup.TmuxBlock(bin, "C-g"), Bin: bin, TmuxVersion: "tmux 3.5a",
		GOOS: "darwin", Bundle: "com.mitchellh.ghostty",
		Look: func(n string) string { return "/opt/homebrew/bin/" + n },
	}
}

func find(cs []Check, name string) Check {
	for _, c := range cs {
		if strings.HasPrefix(c.Name, name) {
			return c
		}
	}
	return Check{Name: "missing:" + name}
}

func TestAllGood(t *testing.T) {
	for _, c := range Run(good(t)) {
		if c.Status != OK {
			t.Fatalf("%+v", c)
		}
	}
}

func TestTmux(t *testing.T) {
	for v, want := range map[string]Status{"tmux 3.2": OK, "tmux 3.10": OK, "tmux next-3.6": OK, "tmux 3.1c": Fail, "tmux 2.9": Fail, "": Fail} {
		e := good(t)
		e.TmuxVersion = v
		if c := find(Run(e), "tmux"); c.Status != want {
			t.Fatalf("%q: %+v", v, c)
		}
	}
}

func TestHooksMissingOrOtherBinary(t *testing.T) {
	e := good(t)
	e.Settings = []byte(`{}`)
	if c := find(Run(e), "Claude Code hooks"); c.Status != Fail || !strings.Contains(c.Fix, "ccm setup") {
		t.Fatalf("%+v", c)
	}
	e.Settings, _, _ = setup.MergeHooks([]byte(`{}`), "/old/ccm")
	if c := find(Run(e), "Claude Code hooks"); c.Status != Fail {
		t.Fatalf("otro binario: %+v", c)
	}
	e.Settings = []byte(`{roto`)
	if c := find(Run(e), "Claude Code hooks"); c.Status != Fail {
		t.Fatalf("roto: %+v", c)
	}
}

func TestTmuxBlock(t *testing.T) {
	e := good(t)
	e.TmuxConf = ""
	if c := find(Run(e), "tmux config"); c.Status != Fail {
		t.Fatalf("%+v", c)
	}
	e.TmuxConf = setup.TmuxBlock(bin, "C-g") + "bind-key -n C-g display-popup \"$HOME/go/bin/ccm\"\n"
	if c := find(Run(e), "tmux config"); c.Status != Warn {
		t.Fatalf("líneas sueltas: %+v", c)
	}
}

func TestNotifier(t *testing.T) {
	e := good(t)
	e.Look = func(string) string { return "" }
	if c := find(Run(e), "notifications"); c.Status != Warn || !strings.Contains(c.Fix, "brew install terminal-notifier") {
		t.Fatalf("%+v", c)
	}
	e.GOOS = "linux"
	if c := find(Run(e), "notifications"); c.Status != Warn || !strings.Contains(c.Fix, "notify-send") {
		t.Fatalf("%+v", c)
	}
}

func TestBundle(t *testing.T) {
	e := good(t)
	e.Bundle = ""
	if c := find(Run(e), "terminal app"); c.Status != Warn {
		t.Fatalf("%+v", c)
	}
	e.GOOS = "linux"
	if c := find(Run(e), "terminal app"); c.Name != "missing:terminal app" {
		t.Fatalf("en linux no aplica: %+v", c)
	}
}

func TestOtherNotifiers(t *testing.T) {
	e := good(t)
	e.Settings, _, _ = setup.MergeHooks([]byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"$HOME/.claude/hooks/notify.sh stop"}]}]}}`), bin)
	if c := find(Run(e), "duplicate notifications"); c.Status != Warn || !strings.Contains(c.Fix, "notify.sh") {
		t.Fatalf("%+v", c)
	}
}

func TestConfigError(t *testing.T) {
	e := good(t)
	e.ConfigErr = errors.New("bad json")
	if c := find(Run(e), "config"); c.Status != Warn {
		t.Fatalf("%+v", c)
	}
}

func TestTmuxBlockWithSpaces(t *testing.T) {
	e := good(t)
	e.Bin = "/Users/John Smith/go/bin/ccm"
	e.TmuxConf = setup.TmuxBlock(e.Bin, "C-g")
	if c := find(Run(e), "tmux config"); c.Status != OK {
		t.Fatalf("%+v", c)
	}
}
