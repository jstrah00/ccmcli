package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/jstrah00/ccmcli/internal/entries"
	"github.com/jstrah00/ccmcli/internal/state"
)

type keyHelp struct{ key, desc string }

var (
	normalHelp = []keyHelp{
		{"j/k", "mover"}, {"⏎", "ir"}, {"/", "filtrar"}, {"r", "renombrar"}, {"dr", "renombrar de cero"}, {"s", "status"}, {"ds", "borrar status"},
		{"dd", "ocultar"}, {"p", "pegar"}, {"U", "restaurar"}, {"c", "cerradas"}, {"q", "salir"},
	}
	filterHelp = []keyHelp{{"⏎", "aceptar"}, {"esc", "limpiar"}, {"↑/↓", "mover"}}
	editHelp   = []keyHelp{{"⏎", "guardar"}, {"esc", "cancelar"}, {"", "vacío borra"}}
)

const minLocWidth = 16

// locWidth: la columna de ubicación crece hasta la más larga (p. ej. un alias),
// sin pasar de la mitad del ancho para que el título siga viéndose.
func (m Model) locWidth() int {
	w := minLocWidth
	for _, e := range m.shown {
		w = max(w, runewidth.StringWidth(location(e)))
	}
	return max(minLocWidth, min(w, m.width/2))
}

const accentColor = "#ff8a4c"

const statusTitle = "Status"

// statusWidth es 0 (sin columna) si ninguna entrada tiene status. Mira la
// lista completa para que la columna no aparezca y desaparezca al filtrar.
func (m Model) statusWidth() int {
	w := 0
	for _, e := range m.list {
		w = max(w, runewidth.StringWidth(e.Status))
	}
	if w == 0 {
		return 0
	}
	return max(runewidth.StringWidth(statusTitle), min(w, m.width/4))
}

// Paleta de Claude Code: acento naranja (más vivo que el del logo para que no
// se vea apagado sobre fondo oscuro), crema para lo seleccionado y grises
// cálidos. Los colores de estado (format.go) quedan semánticos.
var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(logoColor))
	styleHeader = lipgloss.NewStyle().Foreground(lipgloss.Color(logoColor))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a8a095"))
	styleSel    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f4f3ee"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
	styleStatus = lipgloss.NewStyle().Foreground(lipgloss.Color("#e8b08f"))
	styleCols   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#8a8279"))
	styleKey    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor))
	styleAccent = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(accentColor))
	selBg       = lipgloss.Color("#363b47")
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.mode == closedView {
		return m.closedViewRender()
	}
	var b strings.Builder
	for _, l := range m.header() {
		b.WriteString(l + "\n")
	}

	if len(m.list) == 0 {
		b.WriteString(styleDim.Render(fit("  sin sesiones de Claude Code", m.width)) + "\n")
		b.WriteString(styleDim.Render(fit(" q salir", m.width)))
		return b.String()
	}

	rows := m.rows()
	h := m.listHeight()
	locW, stW := m.locWidth(), m.statusWidth()
	b.WriteString(m.columnsLine(locW, stW) + "\n")
	n := 0
	for i := m.offset; i < len(rows) && n < h; i++ {
		switch r := rows[i]; r.entry {
		case rowSpacer:
			b.WriteString("\n")
		case rowHeader:
			b.WriteString(m.sessionHeader(r) + "\n")
		default:
			b.WriteString(m.renderEntry(m.shown[r.entry], r.entry == m.cursor, locW, stW) + "\n")
		}
		n++
	}
	for ; n < h; n++ {
		b.WriteString("\n")
	}
	b.WriteString(styleDim.Render(strings.Repeat("─", max(m.width, 1))) + "\n")
	b.WriteString(m.detailLine() + "\n")
	b.WriteString(m.helpLine())
	return b.String()
}

// prefixW: marca(2) + número(3) + indicador(1) + [remote(2)] + 2.
func (m Model) prefixW() int { return 2 + 3 + 1 + m.remoteW() + 2 }

// remoteW es 0 (sin columna) si ninguna entrada tiene remote-control.
func (m Model) remoteW() int {
	for _, e := range m.list {
		if e.Remote {
			return 2
		}
	}
	return 0
}

func (m Model) columnsLine(locW, stW int) string {
	s := strings.Repeat(" ", m.prefixW()) + fit("Sesión", locW) + "  "
	if stW > 0 {
		s += fit(statusTitle, stW) + "  "
	}
	return styleCols.Render(fit(s+"Conversación", m.width))
}

// sessionHeader: "▌ sesión", con la cantidad de entradas alineada a la derecha.
func (m Model) sessionHeader(r row) string {
	name := styleAccent.Render(" ▌ " + r.header)
	count := fmt.Sprintf("%d", r.count)
	gap := m.width - lipgloss.Width(name) - len(count) - 3
	if gap < 1 {
		return ansi.Truncate(name, m.width, "")
	}
	return name + strings.Repeat(" ", gap+2) + styleDim.Render(count) + " "
}

// Fila: prefijo + ubicación + 2 + [status + 2] + título. La seleccionada
// lleva fondo en todos los segmentos: un reset ANSI intermedio cortaría la
// barra.
func (m Model) renderEntry(e entries.Entry, selected bool, locW, stW int) string {
	paint := func(st lipgloss.Style, s string) string {
		if selected {
			st = st.Background(selBg)
		}
		return st.Render(s)
	}
	text, num := lipgloss.NewStyle(), styleDim
	if selected {
		text, num = styleSel, styleSel
	}
	mark := paint(lipgloss.NewStyle(), "  ")
	if selected {
		mark = paint(styleAccent, "▎ ")
	}
	indStyle, glyph := indicatorParts(e.State, m.frame)
	titleW := m.width - (m.prefixW() + locW + 2)
	line := mark + paint(num, fmt.Sprintf("%2d ", e.Num)) + paint(indStyle.Bold(true), glyph)
	if m.remoteW() > 0 {
		rc := paint(text, " ")
		if e.Remote {
			rc = paint(remoteStyle(m.frame), remoteGlyph)
		}
		line += paint(text, " ") + rc
	}
	line += paint(text, "  ") + paint(text.Bold(true), fit(location(e), locW)) + paint(text, "  ")
	if stW > 0 {
		titleW -= stW + 2
		line += paint(styleStatus, fit(e.Status, stW)) + paint(text, "  ")
	}
	line += paint(text, fit(e.Title, titleW))
	// El prefijo es de ancho fijo: en un popup angosto (o con números de 3
	// dígitos) se pasaría del ancho y la terminal partiría la línea.
	line = ansi.Truncate(line, m.width, "")
	if pad := m.width - lipgloss.Width(line); pad > 0 {
		line += paint(lipgloss.NewStyle(), strings.Repeat(" ", pad))
	}
	return line
}

// helpLine agrega pares tecla/descripción mientras entren, sin cortar uno a
// la mitad.
func (m Model) helpLine() string {
	help := normalHelp
	switch m.mode {
	case filtering:
		help = filterHelp
	case renaming, editingStatus:
		help = editHelp
	case closedView:
		help = closedHelp
	}
	var b strings.Builder
	used := 0
	for _, kh := range help {
		item := styleDim.Render(kh.desc)
		w := runewidth.StringWidth(kh.desc)
		if kh.key != "" {
			item = styleKey.Render(kh.key) + " " + item
			w += runewidth.StringWidth(kh.key) + 1
		}
		if used+w+2 > m.width {
			break
		}
		b.WriteString("  " + item)
		used += w + 2
	}
	return b.String()
}

func (m Model) detailLine() string {
	if m.err != nil {
		return styleErr.Render(fit(" error: "+m.err.Error(), m.width))
	}
	switch m.mode {
	case renaming:
		return styleSel.Render(fit(" nombre: "+m.input+"▏", m.width))
	case editingStatus:
		return styleSel.Render(fit(" status: "+m.input+"▏", m.width))
	}
	e := m.selected()
	if e == nil {
		return ""
	}
	// Último prompt; sin él (sesión abierta antes de los hooks), el título.
	title := e.LastPrompt
	if title == "" {
		title = e.Detail
	}
	if title == "" {
		title = e.Title
	}
	var parts []string
	if title != "" {
		parts = append(parts, title)
	}
	parts = append(parts, stateLabel(e.State))
	if e.Remote {
		parts = append(parts, "remote-control")
	}
	if e.State != state.Unknown && !e.Since.IsZero() {
		parts = append(parts, humanSince(m.d.Now().Sub(e.Since)))
	}
	line := styleDim.Render(" " + strings.Join(parts, " · "))
	if e.Status != "" {
		line = " " + styleStatus.Render(e.Status) + styleDim.Render(" ·") + line
	}
	return ansi.Truncate(line, m.width, "…")
}
