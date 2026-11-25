package closed

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var t0 = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func store(t *testing.T) Store {
	return Store{Path: filepath.Join(t.TempDir(), "state", "closed.json")}
}

func sess(id string) Session {
	return Session{SessionID: id, Cwd: "/p/" + id, LastPrompt: "prompt " + id, ClosedAt: t0}
}

func ids(cs []Session) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.SessionID)
	}
	return out
}

func add(t *testing.T, s Store, cs ...Session) {
	t.Helper()
	for _, c := range cs {
		if err := s.Add(c); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAddOrdersNewestFirst(t *testing.T) {
	s := store(t)
	add(t, s, sess("a"), sess("b"))
	if got := fmt.Sprint(ids(s.List())); got != "[b a]" {
		t.Fatalf("got %s", got)
	}
}

func TestAddDedupes(t *testing.T) {
	s := store(t)
	again := sess("a")
	again.ClosedAt = t0.Add(time.Hour)
	add(t, s, sess("a"), sess("b"), again)
	l := s.List()
	if got := fmt.Sprint(ids(l)); got != "[a b]" {
		t.Fatalf("got %s", got)
	}
	if !l[0].ClosedAt.Equal(t0.Add(time.Hour)) {
		t.Fatalf("quedó la entrada vieja: %+v", l[0])
	}
}

func TestAddCaps(t *testing.T) {
	s := store(t)
	for i := range 55 {
		add(t, s, sess(fmt.Sprintf("s%d", i)))
	}
	l := s.List()
	if len(l) != Max || l[0].SessionID != "s54" {
		t.Fatalf("len=%d primero=%q", len(l), l[0].SessionID)
	}
}

func TestAddSkipsEmpty(t *testing.T) {
	s := store(t)
	add(t, s, Session{LastPrompt: "x"}, Session{SessionID: "a"})
	if l := s.List(); len(l) != 0 {
		t.Fatalf("got %+v", l)
	}
}

func TestTake(t *testing.T) {
	s := store(t)
	add(t, s, sess("a"), sess("b"))
	c, ok, err := s.Take("a")
	if err != nil || !ok || c.SessionID != "a" || c.LastPrompt != "prompt a" {
		t.Fatalf("Take(a) = %+v, %v, %v", c, ok, err)
	}
	if got := fmt.Sprint(ids(s.List())); got != "[b]" {
		t.Fatalf("got %s", got)
	}
	if _, ok, err := s.Take("x"); ok || err != nil {
		t.Fatalf("Take(x) = %v, %v", ok, err)
	}
}

func TestListCorrupt(t *testing.T) {
	s := store(t)
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path, []byte("{no json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if l := s.List(); l != nil {
		t.Fatalf("got %+v", l)
	}
	add(t, s, sess("a"))
	if l := s.List(); len(l) != 1 {
		t.Fatalf("got %+v", l)
	}
}

func TestConcurrentAdds(t *testing.T) {
	s := store(t)
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Add(sess(fmt.Sprintf("s%d", i))); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if l := s.List(); len(l) != 20 {
		t.Fatalf("len = %d, want 20", len(l))
	}
}
