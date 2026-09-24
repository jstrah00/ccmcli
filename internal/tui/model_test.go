package tui

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ccm/internal/entries"
	"ccm/internal/order"
	"ccm/internal/state"
	"ccm/internal/tmux"
)

var t0 = time.Date(2026, 9, 23, 22, 0, 0, 0, time.UTC)

func mk(session string, window int, folder string) entries.Entry {
	path := "/p/" + folder
	p := tmux.PaneInfo{PaneID: fmt.Sprintf("%%%s%d", session, window), Session: session, Window: window, Pane: 1, Command: "claude", Path: path}
	return entries.Entry{Key: order.Key(session, window, 1, path), Pane: p, State: state.Idle, Since: t0, Folder: folder, Title: "t-" + folder}
}

type harness struct {
	live     []entries.Entry
	loadErr  error
	ord      *order.Store
	switched []tmux.PaneInfo
	saves    int
}

func setup(es ...entries.Entry) (*harness, Model) {
	h := &harness{live: es, ord: &order.Store{}}
	m := New(Deps{
		Load:      func() ([]entries.Entry, error) { return h.live, h.loadErr },
		Order:     h.ord,
		SaveOrder: func() error { h.saves++; return nil },
		Switch:    func(p tmux.PaneInfo) error { h.switched = append(h.switched, p); return nil },
		Now:       func() time.Time { return t0.Add(3 * time.Minute) },
	})
	return h, m
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "ctrl+n":
		return tea.KeyMsg{Type: tea.KeyCtrlN}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func send(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		m = send(m, keyMsg(k))
	}
	return m
}

func folders(es []entries.Entry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Folder)
	}
	return out
}

func sel(m Model) string {
	if e := m.selected(); e != nil {
		return e.Folder
	}
	return ""
}

func eqs(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func three() (*harness, Model) { return setup(mk("a", 1, "x"), mk("a", 2, "y"), mk("a", 3, "z")) }

func TestJKMoveAcrossSessionsSkippingHeaders(t *testing.T) {
	_, m := setup(mk("a", 1, "x"), mk("a", 2, "y"), mk("b", 1, "z"))
	if sel(m) != "x" {
		t.Fatalf("inicio en %q", sel(m))
	}
	if m = press(m, "j", "j"); sel(m) != "z" {
		t.Fatalf("j j -> %q", sel(m))
	}
	if m = press(m, "j"); sel(m) != "z" {
		t.Fatalf("j al final -> %q", sel(m))
	}
	if m = press(m, "k"); sel(m) != "y" {
		t.Fatalf("k -> %q", sel(m))
	}
	if m = press(m, "g", "g"); sel(m) != "x" {
		t.Fatalf("gg -> %q", sel(m))
	}
	if m = press(m, "G"); sel(m) != "z" {
		t.Fatalf("G -> %q", sel(m))
	}
}

func TestDDHidesAndPPastesBelow(t *testing.T) {
	h, m := three()
	m = press(m, "d", "d")
	eqs(t, folders(m.list), []string{"y", "z"})
	if sel(m) != "y" || len(h.ord.Hidden) != 1 {
		t.Fatalf("tras dd: sel=%q hidden=%v", sel(m), h.ord.Hidden)
	}
	m = press(m, "j", "p")
	eqs(t, folders(m.list), []string{"y", "z", "x"})
	if sel(m) != "x" || len(h.ord.Hidden) != 0 || h.saves != 2 {
		t.Fatalf("tras p: sel=%q hidden=%v saves=%d", sel(m), h.ord.Hidden, h.saves)
	}
}

func TestShiftPPastesAbove(t *testing.T) {
	_, m := three()
	m = press(m, "d", "d", "P")
	eqs(t, folders(m.list), []string{"x", "y", "z"})
}

func TestNumbersFollowOrder(t *testing.T) {
	_, m := three()
	m = press(m, "d", "d", "j", "p")
	for i, e := range m.list {
		if e.Num != i+1 {
			t.Fatalf("Num de %s = %d, want %d", e.Folder, e.Num, i+1)
		}
	}
}

func TestDDThenQuitLeavesHidden(t *testing.T) {
	h, m := three()
	m = press(m, "d", "d", "q")
	if !m.quitting || len(h.ord.Hidden) != 1 || h.ord.Hidden[0] != mk("a", 1, "x").Key {
		t.Fatalf("quitting=%v hidden=%v", m.quitting, h.ord.Hidden)
	}
}

func TestSecondDDReplacesRegister(t *testing.T) {
	h, m := three()
	m = press(m, "d", "d", "d", "d", "p")
	eqs(t, folders(m.list), []string{"z", "y"})
	if len(h.ord.Hidden) != 1 || h.ord.Hidden[0] != mk("a", 1, "x").Key {
		t.Fatalf("hidden = %v", h.ord.Hidden)
	}
}

func TestURestoresAll(t *testing.T) {
	_, m := three()
	m = press(m, "d", "d", "d", "d", "U")
	eqs(t, folders(m.list), []string{"z", "x", "y"})
}

func TestPendingDExpires(t *testing.T) {
	_, m := three()
	m = press(m, "d")
	m = send(m, timeoutMsg{seq: m.seq})
	m = press(m, "d")
	if len(m.list) != 3 {
		t.Fatalf("d + timeout + d no debería ocultar: %v", folders(m.list))
	}
}

func TestOtherKeyCancelsPendingD(t *testing.T) {
	_, m := three()
	m = press(m, "d", "j", "d")
	if len(m.list) != 3 || sel(m) != "y" {
		t.Fatalf("list=%v sel=%q", folders(m.list), sel(m))
	}
}

func TestEnterSwitches(t *testing.T) {
	h, m := three()
	m = press(m, "j", "enter")
	if len(h.switched) != 1 || h.switched[0].Window != 2 || !m.quitting {
		t.Fatalf("switched=%+v quitting=%v", h.switched, m.quitting)
	}
}

func TestSwitchErrorKeepsOpen(t *testing.T) {
	_, m := three()
	m.d.Switch = func(tmux.PaneInfo) error { return errors.New("boom") }
	m = press(m, "enter")
	if m.quitting || m.err == nil {
		t.Fatalf("quitting=%v err=%v", m.quitting, m.err)
	}
}

func TestDigitJumps(t *testing.T) {
	h, m := three()
	m = press(m, "2")
	if len(h.switched) != 1 || h.switched[0].Window != 2 {
		t.Fatalf("switched = %+v", h.switched)
	}
}

func twelve() (*harness, Model) {
	var es []entries.Entry
	for i := 1; i <= 12; i++ {
		es = append(es, mk("a", i, fmt.Sprintf("f%02d", i)))
	}
	return setup(es...)
}

func TestTwoDigitJump(t *testing.T) {
	h, m := twelve()
	m = press(m, "1", "2")
	if len(h.switched) != 1 || h.switched[0].Window != 12 {
		t.Fatalf("switched = %+v", h.switched)
	}
}

func TestDigitPrefixTimeoutJumpsSingle(t *testing.T) {
	h, m := twelve()
	m = press(m, "1")
	if len(h.switched) != 0 {
		t.Fatal("saltó antes del timeout")
	}
	m = send(m, timeoutMsg{seq: m.seq})
	if len(h.switched) != 1 || h.switched[0].Window != 1 {
		t.Fatalf("switched = %+v", h.switched)
	}
}

func TestDigitThatCannotBePrefixJumpsNow(t *testing.T) {
	h, m := twelve()
	m = press(m, "5")
	if len(h.switched) != 1 || h.switched[0].Window != 5 {
		t.Fatalf("switched = %+v", h.switched)
	}
}

func TestFilterNarrowsAndNumbersStayGlobal(t *testing.T) {
	h, m := setup(mk("a", 1, "alpha"), mk("a", 2, "beta"), mk("b", 1, "gamma"))
	m = press(m, "/", "g", "a", "m", "enter")
	eqs(t, folders(m.shown), []string{"gamma"})
	if m.mode != normal || m.filter != "gam" {
		t.Fatalf("mode=%v filter=%q", m.mode, m.filter)
	}
	m = press(m, "1") // número global: alpha, aunque el filtro no la muestre
	if len(h.switched) != 1 || h.switched[0].Session != "a" || h.switched[0].Window != 1 {
		t.Fatalf("switched = %+v", h.switched)
	}
}

func TestFilterCtrlNMovesWithoutLeaving(t *testing.T) {
	_, m := three()
	m = press(m, "/", "ctrl+n")
	if m.mode != filtering || sel(m) != "y" {
		t.Fatalf("mode=%v sel=%q", m.mode, sel(m))
	}
}

func TestFilterBackspace(t *testing.T) {
	_, m := three()
	m = press(m, "/", "x", "q", "backspace")
	if m.filter != "x" || m.mode != filtering {
		t.Fatalf("filter=%q mode=%v", m.filter, m.mode)
	}
}

func TestEscInFilterClears(t *testing.T) {
	_, m := three()
	m = press(m, "/", "z", "esc")
	if m.mode != normal || m.filter != "" || len(m.shown) != 3 {
		t.Fatalf("mode=%v filter=%q shown=%d", m.mode, m.filter, len(m.shown))
	}
}

func TestEscInNormalClearsFilterThenQuits(t *testing.T) {
	_, m := three()
	m = press(m, "/", "z", "enter", "esc")
	if m.filter != "" || m.quitting {
		t.Fatalf("primer esc: filter=%q quitting=%v", m.filter, m.quitting)
	}
	if m = press(m, "esc"); !m.quitting {
		t.Fatal("segundo esc no cerró")
	}
}

func TestRefreshKeepsCursorOnSameEntry(t *testing.T) {
	h, m := setup(mk("a", 1, "x"), mk("a", 2, "y"))
	m = press(m, "j")
	h.live = append([]entries.Entry{mk("a", 9, "nueva")}, h.live...)
	m = send(m, tickMsg{})
	if sel(m) != "y" {
		t.Fatalf("sel = %q", sel(m))
	}
}

func TestRefreshWhenSelectedDisappears(t *testing.T) {
	h, m := setup(mk("a", 1, "x"), mk("a", 2, "y"), mk("a", 3, "z"))
	m = press(m, "j") // y
	h.live = []entries.Entry{mk("a", 1, "x"), mk("a", 3, "z")}
	if m = send(m, tickMsg{}); sel(m) != "z" {
		t.Fatalf("desaparece del medio: sel = %q", sel(m))
	}
	h.live = []entries.Entry{mk("a", 1, "x")}
	if m = send(m, tickMsg{}); sel(m) != "x" {
		t.Fatalf("desaparece la última: sel = %q", sel(m))
	}
}

func TestLoadErrorKeepsList(t *testing.T) {
	h, m := three()
	h.loadErr = errors.New("tmux caído")
	m = send(m, tickMsg{})
	if m.err == nil || len(m.list) != 3 {
		t.Fatalf("err=%v list=%d", m.err, len(m.list))
	}
}

func TestScrollKeepsCursorVisible(t *testing.T) {
	_, m := twelve()
	m = send(m, tea.WindowSizeMsg{Width: 80, Height: 8}) // 4 filas de lista
	m = press(m, "G")
	// filas: [header, f01..f12] -> cursor en fila 12; offset = 12-4+1 = 9
	if m.offset != 9 {
		t.Fatalf("offset = %d", m.offset)
	}
	m = press(m, "g", "g")
	if m.offset != 0 { // muestra también el encabezado
		t.Fatalf("offset tras gg = %d", m.offset)
	}
}
