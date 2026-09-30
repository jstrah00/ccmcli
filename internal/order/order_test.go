package order

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

var live = []string{"a", "b", "c"}

func eq(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestKey(t *testing.T) {
	if got := Key("mi sesion", 2, 1, "/x/y"); got != "mi sesion:2.1|/x/y" {
		t.Fatalf("Key = %q", got)
	}
}

func TestVisibleOrderThenNewLive(t *testing.T) {
	s := &Store{Order: []string{"c", "dead", "a"}}
	eq(t, s.Visible(live), []string{"c", "a", "b"})
}

func TestVisibleSkipsHidden(t *testing.T) {
	s := &Store{Order: []string{"a", "b"}, Hidden: []string{"b"}}
	eq(t, s.Visible(live), []string{"a", "c"})
}

func TestHide(t *testing.T) {
	s := &Store{}
	s.Hide(live, "b")
	eq(t, s.Visible(live), []string{"a", "c"})
	eq(t, s.Hidden, []string{"b"})
}

func TestPasteBelowAndAbove(t *testing.T) {
	s := &Store{}
	s.Hide(live, "b")
	s.Paste(live, "b", "c", true)
	eq(t, s.Visible(live), []string{"a", "c", "b"})
	eq(t, s.Hidden, nil)

	s.Hide(live, "a")
	s.Paste(live, "a", "b", false)
	eq(t, s.Visible(live), []string{"c", "a", "b"})
}

func TestPasteWithoutAnchorAppends(t *testing.T) {
	s := &Store{}
	s.Hide(live, "a")
	s.Paste(live, "a", "", true)
	eq(t, s.Visible(live), []string{"b", "c", "a"})
}

func TestPasteEmptyKeyIsNoop(t *testing.T) {
	s := &Store{Order: []string{"a"}}
	s.Paste(live, "", "a", true)
	eq(t, s.Order, []string{"a"})
}

func TestRestoreAll(t *testing.T) {
	s := &Store{}
	s.Hide(live, "a")
	s.Hide(live, "c")
	s.RestoreAll(live)
	eq(t, s.Visible(live), []string{"b", "a", "c"})
	eq(t, s.Hidden, nil)
}

func TestDeadKeysAreKept(t *testing.T) {
	s := &Store{Order: []string{"x", "a"}}
	s.Hide([]string{"a", "b"}, "b")
	if !slices.Contains(s.Order, "x") {
		t.Fatalf("se perdió la clave muerta: %v", s.Order)
	}
}

func TestLoadMissingAndCorrupt(t *testing.T) {
	dir := t.TempDir()
	s, err := Load(filepath.Join(dir, "nope.json"))
	if err != nil || len(s.Order) != 0 {
		t.Fatalf("faltante: %+v, %v", s, err)
	}
	p := filepath.Join(dir, "order.json")
	os.WriteFile(p, []byte("{roto"), 0o644)
	s, err = Load(p)
	if err == nil || s == nil || len(s.Order) != 0 {
		t.Fatalf("corrupto: %+v, %v", s, err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "order.json")
	s := &Store{Order: []string{"a", "b"}, Hidden: []string{"c"}}
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	got, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	eq(t, got.Order, s.Order)
	eq(t, got.Hidden, s.Hidden)
}

func TestSetAlias(t *testing.T) {
	s := &Store{}
	s.SetAlias("a", "  backend ")
	if got := s.Alias("a"); got != "backend" {
		t.Fatalf("Alias = %q", got)
	}
	s.SetAlias("a", "  ")
	if _, ok := s.Aliases["a"]; ok {
		t.Fatal("el alias vacío no se borró")
	}
}

func TestAliasesRoundTripAndOldFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "order.json")
	if err := os.WriteFile(p, []byte(`{"order":["a"],"hidden":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(p)
	if err != nil || s.Alias("a") != "" {
		t.Fatalf("Load viejo: %v %v", s, err)
	}
	s.SetAlias("a", "api")
	if err := s.Save(p); err != nil {
		t.Fatal(err)
	}
	s2, err := Load(p)
	if err != nil || s2.Alias("a") != "api" {
		t.Fatalf("round trip: %v %v", s2, err)
	}
}
