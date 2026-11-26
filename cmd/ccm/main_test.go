package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jstrah00/ccmcli/internal/paths"
	"github.com/jstrah00/ccmcli/internal/state"
)

func TestHookNeverWritesOutputAndExitsZero(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	cases := []struct{ name, pane, event, stdin string }{
		{"válido", "%9", "prompt", `{"session_id":"s","cwd":"/x","prompt":"hola"}`},
		{"stdin vacío", "%9", "prompt", ""},
		{"json roto", "%9", "tool-done", "{roto"},
		{"sin pane", "", "prompt", `{"session_id":"s"}`},
		{"unknown event", "%9", "nope", "{}"},
		{"sin evento", "%9", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("TMUX_PANE", c.pane)
			args := []string{"hook"}
			if c.event != "" {
				args = append(args, c.event)
			}
			var out, errb bytes.Buffer
			code := run(args, strings.NewReader(c.stdin), &out, &errb)
			if code != 0 || out.Len() != 0 || errb.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, out.String(), errb.String())
			}
		})
	}
	p, err := state.Store{Dir: paths.PanesDir()}.Load("%9")
	if err != nil || p.State != state.Working || p.SessionID != "s" {
		t.Fatalf("estado final: %+v, %v", p, err)
	}
}

func TestTUIOutsideTmuxFails(t *testing.T) {
	t.Setenv("TMUX", "")
	var out, errb bytes.Buffer
	if code := run(nil, strings.NewReader(""), &out, &errb); code != 1 || !strings.Contains(errb.String(), "tmux") {
		t.Fatalf("code=%d stderr=%q", code, errb.String())
	}
}

func TestUnknownSubcommand(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"nope"}, strings.NewReader(""), &out, &errb); code != 2 {
		t.Fatalf("code = %d", code)
	}
}

func TestNotifyBadInputIsSilent(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"notify"}, strings.NewReader("{roto"), &out, &errb); code != 0 || out.Len() != 0 {
		t.Fatalf("code=%d stdout=%q", code, out.String())
	}
}

func TestUsageListsCommands(t *testing.T) {
	var out, errb bytes.Buffer
	run([]string{"nope"}, strings.NewReader(""), &out, &errb)
	for _, c := range []string{"hook", "seen", "notify", "setup", "doctor"} {
		if !strings.Contains(errb.String(), c) {
			t.Fatalf("usage sin %q: %q", c, errb.String())
		}
	}
}

// Los tests corren el hook de verdad: sin esto, un stop lanzaría el binario de
// test como "ccm notify".
func TestMain(m *testing.M) {
	dir, _ := os.MkdirTemp("", "ccm-test")
	_ = os.MkdirAll(filepath.Join(dir, "ccm"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ccm", "config.json"), []byte(`{"notify":{"banner":false,"sound":false}}`), 0o644)
	os.Setenv("XDG_CONFIG_HOME", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
