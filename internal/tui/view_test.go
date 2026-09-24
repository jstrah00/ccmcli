package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ccm/internal/entries"
	"ccm/internal/state"
)

func sized(m Model, w, h int) Model { return send(m, tea.WindowSizeMsg{Width: w, Height: h}) }

func TestViewHeadersIndicatorsAndDetail(t *testing.T) {
	w := mk("a", 1, "x")
	w.State, w.Detail = state.Waiting, "titulo actual"
	d := mk("a", 2, "y")
	d.State = state.Done
	i := mk("b", 1, "z")
	i.State = state.Idle
	u := mk("b", 2, "u")
	u.State = state.Unknown
	_, m := setup(w, d, i, u)
	v := sized(m, 80, 20).View()
	for _, s := range []string{" a", " b", "●", "✓", "○", "?", " 1 ", " 4 ", "t-x",
		"titulo actual · esperando input · hace 3m"} {
		if !strings.Contains(v, s) {
			t.Errorf("falta %q en:\n%s", s, v)
		}
	}
}

func TestViewDetailFallsBackToRowTitleAndHidesTimeForUnknown(t *testing.T) {
	u := mk("a", 1, "x")
	u.State = state.Unknown
	_, m := setup(u)
	v := sized(m, 80, 10).View()
	if !strings.Contains(v, "t-x · desconocido") || strings.Contains(v, "hace") {
		t.Fatalf("detalle incorrecto:\n%s", v)
	}
}

func TestViewLinesNeverExceedWidth(t *testing.T) {
	e := mk("sesion-con-nombre-largo", 1, "carpeta-con-nombre-larguisimo")
	e.Title = strings.Repeat("título largo 日本語 ", 20)
	e.Detail = e.Title
	_, m := setup(e)
	for _, width := range []int{12, 20, 25, 40} {
		v := sized(m, width, 10).View()
		for _, line := range strings.Split(v, "\n") {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("ancho %d: línea de %d columnas: %q", width, w, line)
			}
		}
	}
}

func TestViewThreeDigitNumberFitsWidth(t *testing.T) {
	e := mk("a", 1, "x")
	e.Num = 100
	_, m := setup(e)
	m = sized(m, 60, 10)
	m.list[0].Num, m.shown[0].Num = 100, 100
	for _, line := range strings.Split(m.View(), "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Fatalf("línea de %d columnas: %q", w, line)
		}
	}
}

func TestViewHeightAndScroll(t *testing.T) {
	var es []entries.Entry
	for i := 1; i <= 10; i++ {
		es = append(es, mk("a", i, fmt.Sprintf("f%02d", i)))
	}
	_, m := setup(es...)
	m = press(sized(m, 80, 8), "G")
	v := m.View()
	if n := strings.Count(v, "\n") + 1; n != 8 {
		t.Fatalf("alto = %d, want 8:\n%s", n, v)
	}
	if !strings.Contains(v, "t-f10") || strings.Contains(v, "t-f01") {
		t.Fatalf("scroll incorrecto:\n%s", v)
	}
}

func TestViewFilterShownInTitle(t *testing.T) {
	_, m := setup(mk("a", 1, "x"))
	m = press(m, "/", "x")
	if !strings.Contains(m.View(), "/x") {
		t.Fatal("el filtro no aparece en el título")
	}
}

func TestViewEmpty(t *testing.T) {
	_, m := setup()
	if !strings.Contains(m.View(), "sin sesiones de Claude Code") {
		t.Fatal("falta el mensaje de lista vacía")
	}
}

func TestViewAfterQuitIsEmpty(t *testing.T) {
	_, m := setup(mk("a", 1, "x"))
	if v := press(m, "q").View(); v != "" {
		t.Fatalf("View tras q = %q", v)
	}
}
