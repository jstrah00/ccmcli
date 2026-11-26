package tui

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/jstrah00/ccmcli/internal/entries"
	"github.com/jstrah00/ccmcli/internal/state"
	"github.com/jstrah00/ccmcli/internal/tmux"
)

func TestHumanSince(t *testing.T) {
	cases := map[time.Duration]string{
		-5 * time.Second: "hace 0s",
		12 * time.Second: "hace 12s",
		3 * time.Minute:  "hace 3m",
		2 * time.Hour:    "hace 2h",
		49 * time.Hour:   "hace 2d",
	}
	for d, want := range cases {
		if got := humanSince(d); got != want {
			t.Errorf("humanSince(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestIndicatorPerState(t *testing.T) {
	cases := map[state.State]string{
		state.Waiting: "●",
		state.Done:    "✔",
		state.Idle:    "○",
		state.Unknown: "?",
	}
	for st, glyph := range cases {
		if got := indicator(st, 0); !strings.Contains(got, glyph) {
			t.Errorf("indicator(%s) = %q, want %q", st, got, glyph)
		}
	}
	if indicator(state.Working, 0) == indicator(state.Working, 1) {
		t.Error("el spinner no avanza entre frames")
	}
}

func TestStateLabel(t *testing.T) {
	if stateLabel(state.Waiting) != "esperando input" || stateLabel(state.Unknown) != "desconocido" {
		t.Fatal("etiquetas incorrectas")
	}
}

func TestFit(t *testing.T) {
	if got := fit("abcdef", 4); got != "abc…" {
		t.Errorf("fit trunca: %q", got)
	}
	if got := fit("ab", 4); got != "ab  " {
		t.Errorf("fit rellena: %q", got)
	}
	if got := fit("日本語テキスト", 6); runewidth.StringWidth(got) != 6 {
		t.Errorf("fit ancho con caracteres anchos: %q", got)
	}
	if got := fit("x", 0); got != "" {
		t.Errorf("fit ancho 0: %q", got)
	}
}

func TestLocation(t *testing.T) {
	mk := func(session string, window int, name, folder string) entries.Entry {
		return entries.Entry{Folder: folder, Pane: tmux.PaneInfo{Session: session, Window: window, WindowName: name, Command: "claude"}}
	}
	cases := []struct {
		e    entries.Entry
		want string
	}{
		// window con nombre automático y carpeta igual a la sesión: solo el índice
		{mk("stow_dotfiles", 2, "claude", "stow_dotfiles"), ":2"},
		{mk("uyquemoderno-web", 1, "claude", "public"), ":1 public"},
		{mk("elpia", 3, "api", "elpia"), ":3 api"},
		{mk("elpia", 3, "api", "docs"), ":3 api docs"},
		{mk("elpia", 1, "", "elpia"), ":1"},
		{func() entries.Entry { e := mk("elpia", 3, "api", "docs"); e.Alias = "mi nombre"; return e }(), "mi nombre"},
	}
	for _, c := range cases {
		if got := location(c.e); got != c.want {
			t.Errorf("location(%+v) = %q, want %q", c.e.Pane, got, c.want)
		}
	}
}

func TestBuildRowsRepeatsHeaderWhenNotContiguous(t *testing.T) {
	mk := func(s string) entries.Entry { return entries.Entry{Pane: tmux.PaneInfo{Session: s}} }
	got := buildRows([]entries.Entry{mk("a"), mk("a"), mk("b"), mk("a")}, false)
	want := []row{
		{header: "a", count: 2, entry: rowHeader}, {entry: 0}, {entry: 1},
		{entry: rowSpacer}, {header: "b", count: 1, entry: rowHeader}, {entry: 2},
		{entry: rowSpacer}, {header: "a", count: 1, entry: rowHeader}, {entry: 3},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

// Logo en el naranja de Claude Code, sin degradé.
func TestLogoColorIsClaudeOrange(t *testing.T) {
	if logoColor != "#d97757" {
		t.Fatalf("logoColor = %q", logoColor)
	}
}

// Todo el acento de la UI va en el naranja de Claude, igual que el logo.
func TestAccentStylesUseClaudeOrange(t *testing.T) {
	want := lipgloss.Color(logoColor)
	for name, st := range map[string]lipgloss.Style{"title": styleTitle, "header": styleHeader} {
		if got := st.GetForeground(); got != want {
			t.Errorf("%s: foreground = %v, want %v", name, got, want)
		}
	}
}
