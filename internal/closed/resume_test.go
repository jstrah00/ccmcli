package closed

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"ccm/internal/tmux"
)

type fakeTmux struct {
	sessions map[string]bool
	current  string
	calls    []string
}

var newPane = tmux.PaneInfo{PaneID: "%9", Session: "x", Window: 4, Pane: 1}

func (f *fakeTmux) HasSession(n string) bool        { return f.sessions[n] }
func (f *fakeTmux) CurrentSession() (string, error) { return f.current, nil }
func (f *fakeTmux) NewWindow(session, cwd string) (tmux.PaneInfo, error) {
	f.calls = append(f.calls, fmt.Sprintf("new %s %s", session, cwd))
	return newPane, nil
}
func (f *fakeTmux) SendLine(id, line string) error {
	f.calls = append(f.calls, fmt.Sprintf("send %s %s", id, line))
	return nil
}
func (f *fakeTmux) Switch(p tmux.PaneInfo) error {
	f.calls = append(f.calls, "switch "+p.PaneID)
	return nil
}

func TestResumeInOriginalSession(t *testing.T) {
	cwd := t.TempDir()
	f := &fakeTmux{sessions: map[string]bool{"work": true}, current: "now"}
	p, err := Resume(Session{SessionID: "s1", Cwd: cwd, TmuxSession: "work"}, f)
	if err != nil || p != newPane {
		t.Fatalf("Resume = %+v, %v", p, err)
	}
	want := []string{"new work " + cwd, "send %9 claude --resume s1", "switch %9"}
	if !reflect.DeepEqual(f.calls, want) {
		t.Fatalf("got %v\nwant %v", f.calls, want)
	}
}

func TestResumeFallsBackToCurrent(t *testing.T) {
	cwd := t.TempDir()
	for _, sess := range []string{"gone", ""} {
		f := &fakeTmux{sessions: map[string]bool{"": true}, current: "now"}
		if _, err := Resume(Session{SessionID: "s1", Cwd: cwd, TmuxSession: sess}, f); err != nil {
			t.Fatal(err)
		}
		if f.calls[0] != "new now "+cwd {
			t.Fatalf("TmuxSession %q: %v", sess, f.calls)
		}
	}
}

func TestResumeMissingCwd(t *testing.T) {
	f := &fakeTmux{current: "now"}
	_, err := Resume(Session{SessionID: "s1", Cwd: filepath.Join(t.TempDir(), "no")}, f)
	if err == nil || len(f.calls) != 0 {
		t.Fatalf("err = %v, calls = %v", err, f.calls)
	}
}

func TestResumeBadSessionID(t *testing.T) {
	f := &fakeTmux{current: "now"}
	for _, id := range []string{"x; rm -rf ~", "", "a b", "$(x)"} {
		_, err := Resume(Session{SessionID: id, Cwd: t.TempDir()}, f)
		if !errors.Is(err, ErrBadSession) || len(f.calls) != 0 {
			t.Fatalf("%q: err = %v, calls = %v", id, err, f.calls)
		}
	}
}
