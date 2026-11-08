package entries

import (
	"testing"
	"time"

	"ccm/internal/state"
	"ccm/internal/tmux"
	"ccm/internal/transcript"
)

var t0 = time.Date(2026, 9, 23, 22, 0, 0, 0, time.UTC)

var panes = []tmux.PaneInfo{
	{PaneID: "%1", Session: "s", Window: 1, Pane: 1, Command: "claude", Path: "/h/proj"},
	{PaneID: "%2", Session: "s", Window: 1, Pane: 2, Command: "claude", Path: "/h/proj"},
	{PaneID: "%3", Session: "s", Window: 2, Pane: 1, Command: "zsh", Path: "/h/proj"},
}

func titles(sid, path string) transcript.Titles {
	if sid == "S1" && path == "/t1" {
		return transcript.Titles{FirstAITitle: "first", LastAITitle: "last"}
	}
	return transcript.Titles{}
}

func TestCollect(t *testing.T) {
	states := map[string]state.Pane{
		"%1": {PaneID: "%1", SessionID: "S1", TranscriptPath: "/t1", Cwd: "/h/proj", State: state.Waiting, Since: t0, LastPrompt: "hacé X"},
	}
	es := Collect(panes, states, titles, map[string]bool{"S1": true})
	if es[0].LastPrompt != "hacé X" {
		t.Fatalf("LastPrompt = %q", es[0].LastPrompt)
	}
	if len(es) != 2 {
		t.Fatalf("esperaba 2 entradas (solo claude), got %d", len(es))
	}
	a, b := es[0], es[1]
	if a.State != state.Waiting || !a.Since.Equal(t0) || a.Title != "first" || a.Detail != "last" || a.Folder != "proj" {
		t.Fatalf("entrada con estado: %+v", a)
	}
	if !a.Remote {
		t.Fatalf("entrada con remote-control sin Remote: %+v", a)
	}
	if b.State != state.Unknown || b.Title != "" || b.Folder != "proj" || b.Remote {
		t.Fatalf("entrada sin estado: %+v", b)
	}
	// Dos CC en la misma window y mismo cwd: claves distintas.
	if a.Key == b.Key {
		t.Fatalf("claves iguales: %q", a.Key)
	}
}

func TestArrangeNumbers(t *testing.T) {
	es := Collect(panes, nil, titles, nil)
	got := Arrange(es, []string{es[1].Key, es[0].Key})
	if got[0].Pane.PaneID != "%2" || got[0].Num != 1 || got[1].Pane.PaneID != "%1" || got[1].Num != 2 {
		t.Fatalf("got %+v", got)
	}
}

func TestPurgeDeletesDeadPanes(t *testing.T) {
	s := state.Store{Dir: t.TempDir()}
	_ = s.Save(state.Pane{PaneID: "%1", State: state.Idle, Since: t0})
	_ = s.Save(state.Pane{PaneID: "%99", State: state.Idle, Since: t0})
	all, _ := s.All()
	Purge(s, panes, all)
	after, _ := s.All()
	if _, ok := after["%99"]; ok || len(after) != 1 {
		t.Fatalf("after = %+v", after)
	}
}

func TestCollectAcceptsVersionNamedBinary(t *testing.T) {
	ps := []tmux.PaneInfo{
		{PaneID: "%1", Session: "s", Window: 1, Pane: 1, Command: "claude", Path: "/h/proj"},
		{PaneID: "%2", Session: "s", Window: 2, Pane: 1, Command: "2.1.280", Path: "/h/proj"},
		{PaneID: "%3", Session: "s", Window: 3, Pane: 1, Command: "zsh", Path: "/h/proj"},
		{PaneID: "%4", Session: "s", Window: 4, Pane: 1, Command: "node", Path: "/h/proj"},
	}
	es := Collect(ps, nil, titles, nil)
	if len(es) != 2 {
		t.Fatalf("esperaba 2 entradas (claude + binario versionado), got %d", len(es))
	}
	if es[1].Pane.PaneID != "%2" {
		t.Fatalf("falta el pane del binario versionado: %+v", es)
	}
}
