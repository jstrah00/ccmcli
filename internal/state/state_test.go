package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 23, 22, 0, 0, 0, time.UTC)

func samePane(t *testing.T, got, want Pane) {
	t.Helper()
	if got.PaneID != want.PaneID || got.SessionID != want.SessionID || got.Cwd != want.Cwd ||
		got.TranscriptPath != want.TranscriptPath || got.State != want.State || !got.Since.Equal(want.Since) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestApplyChangesSinceOnlyOnStateChange(t *testing.T) {
	p := Apply(Pane{PaneID: "%1"}, Working, t0)
	if p.State != Working || !p.Since.Equal(t0) {
		t.Fatalf("primer apply: %+v", p)
	}
	p = Apply(p, Working, t0.Add(time.Minute))
	if !p.Since.Equal(t0) {
		t.Fatalf("since cambió sin cambiar el estado: %v", p.Since)
	}
	p = Apply(p, Done, t0.Add(2*time.Minute))
	if p.State != Done || !p.Since.Equal(t0.Add(2*time.Minute)) {
		t.Fatalf("cambio de estado: %+v", p)
	}
}

func TestStoreRoundTripAndFileName(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	p := Pane{PaneID: "%41", SessionID: "abc", Cwd: "/x", TranscriptPath: "/x/t.jsonl", State: Waiting, Since: t0}
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "41.json")); err != nil {
		t.Fatalf("esperaba 41.json: %v", err)
	}
	got, err := s.Load("%41")
	if err != nil {
		t.Fatal(err)
	}
	samePane(t, got, p)
}

func TestStoreDeleteMissingIsNotError(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Delete("%9"); err != nil {
		t.Fatal(err)
	}
}

func TestStoreDeleteRemoves(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	_ = s.Save(Pane{PaneID: "%1", State: Idle, Since: t0})
	if err := s.Delete("%1"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load("%1"); err == nil {
		t.Fatal("el archivo sigue existiendo")
	}
}

func TestStoreAllSkipsCorruptAndTemp(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	if err := s.Save(Pane{PaneID: "%1", State: Idle, Since: t0}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(s.Dir, "2.json"), []byte("{no json"), 0o644)
	os.WriteFile(filepath.Join(s.Dir, ".tmp-123"), []byte("x"), 0o644)
	all, err := s.All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all["%1"].State != Idle {
		t.Fatalf("All = %+v", all)
	}
}

func TestStoreAllMissingDir(t *testing.T) {
	s := Store{Dir: filepath.Join(t.TempDir(), "nope")}
	all, err := s.All()
	if err != nil || len(all) != 0 {
		t.Fatalf("All = %v, %v", all, err)
	}
}
