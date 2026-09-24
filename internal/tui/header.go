package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ccm/internal/state"
)

// "CCM CLI" con medios bloques: 2 líneas de alto, 24 columnas.
var logo = [2]string{
	"▄▀▀ ▄▀▀ █▄ ▄█  ▄▀▀ █   █",
	"▀▄▄ ▀▄▄ █ ▀ █  ▀▄▄ █▄▄ █",
}

// Naranja de Claude Code.
const logoColor = "#d97757"

var styleLogo = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(logoColor))

// bigHeader: el logo solo va si sobra lugar; si no, una línea compacta
// para no comerle filas a la lista.
func (m *Model) bigHeader() bool { return m.height >= 14 && m.width >= 44 }

func (m *Model) headerLines() int {
	if m.bigHeader() {
		return 4 // 2 del logo + resumen + 1 en blanco
	}
	return 1
}

func (m Model) header() []string {
	if m.bigHeader() {
		return []string{
			center(styleLogo.Render(logo[0]), m.width),
			center(styleLogo.Render(logo[1]), m.width),
			center(m.status(), m.width),
			"",
		}
	}
	left := " " + styleTitle.Render("CCM") + styleDim.Render(" cli")
	return []string{spread(left, m.status(), m.width)}
}

// status: total (o filtro) + contadores por estado, si no se está filtrando.
func (m Model) status() string {
	st := m.topRight()
	if s := m.summary(); s != "" && m.mode != filtering && m.filter == "" {
		st += styleDim.Render(" · ") + s
	}
	return st
}

// center centra s en exactamente width columnas.
func center(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return ansi.Truncate(s, width, "")
	}
	left := (width - w) / 2
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", width-w-left)
}

// topRight: el filtro si hay uno; si no, el total de sesiones.
func (m Model) topRight() string {
	switch {
	case m.mode == filtering:
		return styleTitle.Render("/" + m.filter + "▏")
	case m.filter != "":
		return styleTitle.Render("/" + m.filter)
	case len(m.list) == 1:
		return styleDim.Render("1 sesión")
	default:
		return styleDim.Render(fmt.Sprintf("%d sesiones", len(m.list)))
	}
}

// summary cuenta solo los estados que piden atención o están corriendo.
func (m Model) summary() string {
	count := map[state.State]int{}
	for _, e := range m.list {
		count[e.State]++
	}
	var parts []string
	for _, st := range []state.State{state.Waiting, state.Done, state.Working} {
		if n := count[st]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, indicator(st, m.frame)))
		}
	}
	return strings.Join(parts, "  ")
}

// spread pone left a la izquierda y right alineado a la derecha (con un
// espacio de margen), en exactamente width columnas.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 1
	if gap < 1 {
		return ansi.Truncate(left+" "+right, width, "")
	}
	return left + strings.Repeat(" ", gap) + right + " "
}
