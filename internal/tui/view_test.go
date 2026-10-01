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
	for _, s := range []string{" a", " b", "●", "✔", "○", "?", " 1 ", " 4 ", "t-x",
		"titulo actual · esperando input · hace 3m"} {
		if !strings.Contains(v, s) {
			t.Errorf("falta %q en:\n%s", s, v)
		}
	}
}

func TestViewDetailShowsLastPrompt(t *testing.T) {
	e := mk("a", 1, "x")
	e.State, e.Detail, e.LastPrompt = state.Working, "titulo actual", "arreglá el login"
	_, m := setup(e)
	v := sized(m, 80, 10).View()
	if !strings.Contains(v, "arreglá el login · trabajando · hace 3m") || strings.Contains(v, "titulo actual") {
		t.Fatalf("detalle incorrecto:\n%s", v)
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

func lines(v string) []string { return strings.Split(v, "\n") }

func states4() (*harness, Model) {
	w := mk("a", 1, "w")
	w.State = state.Waiting
	d1 := mk("a", 2, "d1")
	d1.State = state.Done
	d2 := mk("a", 3, "d2")
	d2.State = state.Done
	k := mk("a", 4, "k")
	k.State = state.Working
	return setup(w, d1, d2, k, mk("a", 5, "i"))
}

func TestHeaderBigShowsCenteredLogoAndSummaryBelow(t *testing.T) {
	_, m := states4()
	l := lines(sized(m, 80, 20).View())
	pad := strings.Repeat(" ", (80-24)/2)
	if !strings.HasPrefix(l[0], pad+"▄▀▀ ▄▀▀ █▄ ▄█") || !strings.HasPrefix(l[1], pad+"▀▄▄ ▀▄▄ █ ▀ █") {
		t.Fatalf("logo no centrado:\n%q\n%q", l[0], l[1])
	}
	sum := l[2]
	for _, s := range []string{"5 sesiones", "1 ●", "2 ✔", "1 " + spinnerFrames[0]} {
		if !strings.Contains(sum, s) {
			t.Fatalf("falta %q en el resumen: %q", s, sum)
		}
	}
	if strings.Contains(sum, "○") {
		t.Fatalf("idle no va en el resumen: %q", sum)
	}
	// centrado: el margen izquierdo y el derecho difieren a lo sumo en 1
	left := len(sum) - len(strings.TrimLeft(sum, " "))
	right := len(sum) - len(strings.TrimRight(sum, " "))
	if d := left - right; d < -1 || d > 1 {
		t.Fatalf("resumen no centrado (izq %d, der %d): %q", left, right, sum)
	}
	if strings.TrimSpace(l[3]) != "" || !strings.HasPrefix(strings.TrimSpace(l[4]), "Sesión") ||
		strings.Contains(l[5], "─") ||
		!strings.HasPrefix(strings.TrimSpace(l[5]), "▌ a ") || !strings.HasSuffix(strings.TrimSpace(l[5]), " 5") {
		t.Fatalf("esperaba línea en blanco, columnas y encabezado:\n%q\n%q\n%q", l[3], l[4], l[5])
	}
}

func TestHeaderSingular(t *testing.T) {
	_, m := setup(mk("a", 1, "x"))
	if v := sized(m, 80, 20).View(); !strings.Contains(v, "1 sesión") || strings.Contains(v, "1 sesiones") {
		t.Fatalf("singular:\n%s", v)
	}
}

func TestHeaderFilterReplacesTotal(t *testing.T) {
	_, m := states4()
	m = press(sized(m, 80, 20), "/", "d")
	l := lines(m.View())
	if !strings.Contains(l[2], "/d") || strings.Contains(l[2], "sesiones") {
		t.Fatalf("filtro en el header: %q", l[2])
	}
}

func TestHeaderCompactWhenSmall(t *testing.T) {
	_, m := states4()
	for _, size := range [][2]int{{80, 13}, {43, 20}} {
		v := sized(m, size[0], size[1]).View()
		l := lines(v)
		if strings.Contains(v, "▄▀▀") || !strings.Contains(l[0], "CCM") {
			t.Fatalf("%v: esperaba header compacto: %q", size, l[0])
		}
		if n := len(l); n != size[1] {
			t.Fatalf("%v: alto = %d", size, n)
		}
	}
}

func TestHeaderBigKeepsHeightAndWidth(t *testing.T) {
	_, m := states4()
	for _, size := range [][2]int{{44, 14}, {80, 20}} {
		l := lines(sized(m, size[0], size[1]).View())
		if len(l) != size[1] {
			t.Fatalf("%v: alto = %d", size, len(l))
		}
		for _, line := range l {
			if w := lipgloss.Width(line); w > size[0] {
				t.Fatalf("%v: línea de %d columnas: %q", size, w, line)
			}
		}
	}
}

func TestHeaderShownWhenEmpty(t *testing.T) {
	_, m := setup()
	if v := sized(m, 80, 20).View(); !strings.Contains(v, "▄▀▀") || !strings.Contains(v, "0 sesiones") {
		t.Fatalf("vacío sin header:\n%s", v)
	}
}

func TestViewAliasReplacesLocationAndRenameInput(t *testing.T) {
	e := mk("a", 1, "x")
	h, m := setup(e)
	h.ord.SetAlias(e.Key, "backend")
	m = sized(send(m, tickMsg{}), 80, 10)
	v := m.View()
	if !strings.Contains(v, "backend") || strings.Contains(v, ":1") {
		t.Fatalf("alias no reemplaza la ubicación:\n%s", v)
	}
	v = press(m, "r", "z").View()
	if !strings.Contains(v, "nombre: backendz▏") || !strings.Contains(v, "guardar") {
		t.Fatalf("falta el input de renombre:\n%s", v)
	}
}

func TestViewLongAliasWidensLocation(t *testing.T) {
	e := mk("a", 1, "x")
	h, m := setup(e)
	h.ord.SetAlias(e.Key, "un nombre bastante largo")
	v := sized(send(m, tickMsg{}), 80, 10).View()
	if !strings.Contains(v, "un nombre bastante largo  t-x") {
		t.Fatalf("alias truncado:\n%s", v)
	}
}

func TestViewStatusColumnOnlyWhenSomeHasStatus(t *testing.T) {
	a, b := mk("a", 1, "x"), mk("a", 2, "y")
	h, m := setup(a, b)
	m = sized(m, 80, 12)
	if v := m.View(); strings.Contains(v, "Status") || !strings.Contains(v, "Conversación") {
		t.Fatalf("columna status sin statuses:\n%s", v)
	}
	h.ord.SetStatus(b.Key, "en review")
	v := send(m, tickMsg{}).View()
	if !strings.Contains(v, "Status") || !strings.Contains(v, "en review  t-y") {
		t.Fatalf("falta la columna status:\n%s", v)
	}
	if v := press(send(m, tickMsg{}), "j", "s", "!").View(); !strings.Contains(v, "status: en review!▏") {
		t.Fatalf("falta el input de status:\n%s", v)
	}
}

func TestViewLongStatusFitsWidth(t *testing.T) {
	e := mk("a", 1, "x")
	h, m := setup(e)
	h.ord.SetStatus(e.Key, strings.Repeat("status larguísimo 日本 ", 10))
	m = send(m, tickMsg{})
	for _, width := range []int{12, 30, 80} {
		for _, line := range lines(sized(m, width, 12).View()) {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("ancho %d: línea de %d columnas: %q", width, w, line)
			}
		}
	}
}
