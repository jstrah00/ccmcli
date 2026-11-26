package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/jstrah00/ccmcli/internal/closed"
	"github.com/jstrah00/ccmcli/internal/entries"
	"github.com/jstrah00/ccmcli/internal/order"
	"github.com/jstrah00/ccmcli/internal/tmux"
	"github.com/jstrah00/ccmcli/internal/transcript"
)

type closedHarness struct {
	*harness
	closed    []closed.Session
	resumed   []closed.Session
	resumeErr error
}

var resumedPane = tmux.PaneInfo{PaneID: "%90", Session: "personal", Window: 7, Pane: 1, Path: "/p/ccmcli"}

func threeClosed() []closed.Session {
	now := t0.Add(3 * time.Minute)
	return []closed.Session{
		{SessionID: "a", TmuxSession: "work", Cwd: "/p/api", LastPrompt: "arreglá el login", ClosedAt: now.Add(-4 * time.Minute)},
		{SessionID: "b", TmuxSession: "personal", Cwd: "/p/ccmcli", LastPrompt: "rediseñar el header", ClosedAt: now.Add(-time.Hour), Alias: "popup", Status: "wip", TranscriptPath: "/t/b"},
		{SessionID: "c", TmuxSession: "work", Cwd: "/p/web", LastPrompt: "subí el deploy", ClosedAt: now.Add(-48 * time.Hour)},
	}
}

func setupClosed(cs []closed.Session) (*closedHarness, Model) {
	h := &closedHarness{harness: &harness{live: nil, ord: &order.Store{}}, closed: cs}
	h.live = append(h.live, mk("a", 1, "x"))
	m := New(Deps{
		Load:      func() ([]entries.Entry, error) { return h.live, nil },
		Order:     h.ord,
		SaveOrder: func() error { h.saves++; return nil },
		Switch:    func(tmux.PaneInfo) error { return nil },
		Now:       func() time.Time { return t0.Add(3 * time.Minute) },
		Closed:    func() []closed.Session { return h.closed },
		Titles: func(sid, path string) transcript.Titles {
			if sid == "b" {
				return transcript.Titles{FirstAITitle: "Header nuevo", LastAITitle: "Header nuevo"}
			}
			return transcript.Titles{}
		},
		Resume: func(c closed.Session) (tmux.PaneInfo, error) {
			h.resumed = append(h.resumed, c)
			return resumedPane, h.resumeErr
		},
	})
	return h, m
}

func ctrl(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

func typeText(m Model, s string) Model {
	for _, r := range s {
		m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func closedIDs(m Model) string {
	var out []string
	for _, r := range m.closedShown {
		out = append(out, r.s.SessionID)
	}
	return strings.Join(out, " ")
}

func TestClosedOpens(t *testing.T) {
	_, m := setupClosed(threeClosed())
	m = press(m, "c")
	if m.mode != closedView || closedIDs(m) != "a b c" {
		t.Fatalf("mode=%v filas=%q", m.mode, closedIDs(m))
	}
	v := m.View()
	for _, want := range []string{"Cerradas · 3", "arreglá el login", "Header nuevo", "popup", "hace 4m"} {
		if !strings.Contains(v, want) {
			t.Errorf("la vista no contiene %q:\n%s", want, v)
		}
	}
}

func TestClosedFuzzy(t *testing.T) {
	_, m := setupClosed(threeClosed())
	m = typeText(press(m, "c"), "hdr")
	if closedIDs(m) != "b" {
		t.Fatalf("hdr -> %q", closedIDs(m))
	}
	m = press(m, "backspace", "backspace", "backspace")
	if closedIDs(m) != "a b c" {
		t.Fatalf("sin query -> %q", closedIDs(m))
	}
	m = typeText(m, "zzz")
	if closedIDs(m) != "" || !strings.Contains(m.View(), "sin resultados") {
		t.Fatalf("zzz -> %q\n%s", closedIDs(m), m.View())
	}
}

func TestClosedSearchesPromptAndAlias(t *testing.T) {
	_, m := setupClosed(threeClosed())
	if got := closedIDs(typeText(press(m, "c"), "deploy")); got != "c" {
		t.Fatalf("deploy -> %q", got)
	}
	if got := closedIDs(typeText(press(m, "c"), "popup")); got != "b" {
		t.Fatalf("popup -> %q", got)
	}
}

func TestClosedTypingJKIsQuery(t *testing.T) {
	_, m := setupClosed(threeClosed())
	m = press(m, "c", "j")
	if m.query != "j" || m.ccursor != 0 {
		t.Fatalf("query=%q cursor=%d", m.query, m.ccursor)
	}
}

func TestClosedMove(t *testing.T) {
	_, m := setupClosed(threeClosed())
	m = press(m, "c", "down", "ctrl+n")
	if m.ccursor != 2 {
		t.Fatalf("cursor = %d", m.ccursor)
	}
	if m = press(m, "down"); m.ccursor != 2 {
		t.Fatalf("pasó el final: %d", m.ccursor)
	}
	if m = send(m, ctrl(tea.KeyCtrlP)); m.ccursor != 1 {
		t.Fatalf("ctrl+p -> %d", m.ccursor)
	}
	if m = send(m, ctrl(tea.KeyCtrlK)); m.ccursor != 0 {
		t.Fatalf("ctrl+k -> %d", m.ccursor)
	}
	if m = send(m, ctrl(tea.KeyCtrlJ)); m.ccursor != 1 {
		t.Fatalf("ctrl+j -> %d", m.ccursor)
	}
}

func TestClosedEsc(t *testing.T) {
	_, m := setupClosed(threeClosed())
	m = press(m, "c", "x", "esc")
	if m.query != "" || m.mode != closedView {
		t.Fatalf("query=%q mode=%v", m.query, m.mode)
	}
	if m = press(m, "esc"); m.mode != normal || m.quitting {
		t.Fatalf("mode=%v quitting=%v", m.mode, m.quitting)
	}
}

func TestClosedResume(t *testing.T) {
	h, m := setupClosed(threeClosed())
	m = press(m, "c", "down", "enter")
	if len(h.resumed) != 1 || h.resumed[0].SessionID != "b" {
		t.Fatalf("resumed = %+v", h.resumed)
	}
	key := order.Key("personal", 7, 1, "/p/ccmcli")
	if h.ord.Alias(key) != "popup" || h.ord.Status(key) != "wip" || h.saves != 1 {
		t.Fatalf("alias=%q status=%q saves=%d", h.ord.Alias(key), h.ord.Status(key), h.saves)
	}
	if !m.quitting {
		t.Fatal("no cerró el popup")
	}
}

func TestClosedResumeError(t *testing.T) {
	h, m := setupClosed(threeClosed())
	h.resumeErr = errors.New("la carpeta ya no existe: /p/api")
	m = press(m, "c", "enter")
	if m.err == nil || m.mode != closedView || m.quitting || h.saves != 0 {
		t.Fatalf("err=%v mode=%v quitting=%v", m.err, m.mode, m.quitting)
	}
	if !strings.Contains(m.View(), "la carpeta ya no existe") {
		t.Fatalf("el error no se ve:\n%s", m.View())
	}
}

func TestClosedEmpty(t *testing.T) {
	h, m := setupClosed(nil)
	m = press(m, "c")
	if !strings.Contains(m.View(), "todavía no hay sesiones cerradas") {
		t.Fatalf("vista:\n%s", m.View())
	}
	if m = press(m, "enter"); len(h.resumed) != 0 || m.quitting {
		t.Fatal("enter con lista vacía hizo algo")
	}
}

func TestClosedNarrow(t *testing.T) {
	for _, size := range [][2]int{{20, 8}, {8, 5}, {1, 1}, {120, 40}} {
		_, m := setupClosed(threeClosed())
		m = sized(typeText(press(m, "c"), "a"), size[0], size[1])
		lines := strings.Split(m.View(), "\n")
		if len(lines) > max(size[1], 6) {
			t.Errorf("%v: %d líneas", size, len(lines))
		}
		for _, line := range lines {
			if w := lipgloss.Width(line); w > size[0] {
				t.Errorf("%v: línea de ancho %d: %q", size, w, line)
			}
		}
	}
}

func TestClosedHighlightsMatches(t *testing.T) {
	_, m := setupClosed(threeClosed())
	m = typeText(press(m, "c"), "deploy")
	if got := m.closedShown[0].matched; len(got) != len("deploy") {
		t.Fatalf("matched = %v", got)
	}
	// El resaltado no puede comerse ni duplicar texto, tampoco con acentos.
	if !strings.Contains(ansi.Strip(m.View()), "subí el deploy") {
		t.Fatalf("vista:\n%s", ansi.Strip(m.View()))
	}
}

func TestClosedNilDeps(t *testing.T) {
	_, m := three()
	if m = press(m, "c"); m.mode != normal {
		t.Fatalf("mode = %v", m.mode)
	}
}
