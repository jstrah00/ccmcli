package tmux

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestParsePanes(t *testing.T) {
	out := "%1\tmi sesion.v2\t3\t2\tclaude\t/home/u/p\t1\t1\tmi window\n" +
		"%2\tb\t1\t1\tzsh\t/tmp\t0\t0\tzsh\n"
	got, err := ParsePanes(out)
	if err != nil {
		t.Fatal(err)
	}
	want := []PaneInfo{
		{PaneID: "%1", Session: "mi sesion.v2", Window: 3, Pane: 2, Command: "claude", Path: "/home/u/p", WindowActive: true, SessionAttached: true, WindowName: "mi window"},
		{PaneID: "%2", Session: "b", Window: 1, Pane: 1, Command: "zsh", Path: "/tmp", WindowName: "zsh"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParsePanesAttachedCountAboveOne(t *testing.T) {
	got, err := ParsePanes("%1\ts\t1\t1\tclaude\t/x\t1\t2\tw\n")
	if err != nil || !got[0].SessionAttached {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestParsePanesEmpty(t *testing.T) {
	got, err := ParsePanes("")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestParsePanesBadLine(t *testing.T) {
	if _, err := ParsePanes("%1\tfaltan campos\n"); err == nil {
		t.Fatal("esperaba error")
	}
}

// Targets exactos: sin "=", "a" matchearía por prefijo con "a2w".
func TestSwitchArgsUseExactTargets(t *testing.T) {
	got := switchArgs(PaneInfo{PaneID: "%7", Session: "a", Window: 2})
	want := [][]string{
		{"switch-client", "-t", "=a"},
		{"select-window", "-t", "=a:2"},
		{"select-pane", "-t", "%7"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestExecAgainstRealServer(t *testing.T) {
	bin, err := Find()
	if err != nil {
		t.Skip("tmux no instalado")
	}
	e := Exec{Bin: bin, Socket: fmt.Sprintf("ccmtest%d", os.Getpid())}
	if _, err := e.run("-f", "/dev/null", "new-session", "-d", "-s", "prueba x", "-c", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.run("kill-server") })

	ps, err := e.ListPanes()
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 1 || ps[0].Session != "prueba x" || !strings.HasPrefix(ps[0].PaneID, "%") {
		t.Fatalf("ListPanes = %+v", ps)
	}
	active, err := e.PaneActive(ps[0].PaneID)
	if err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("sin cliente attached no debería contar como activa")
	}
}
