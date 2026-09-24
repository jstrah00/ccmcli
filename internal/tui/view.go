package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ccm/internal/entries"
	"ccm/internal/state"
)

const helpText = "j/k mover · ⏎ ir · / filtrar · dd ocultar · p pegar · U restaurar · q salir"

const folderWidth = 16

var (
	styleTitle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	styleHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7"))
	styleDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	styleSel    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c0caf5"))
	styleErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))
)

func (m Model) View() string {
	if m.quitting {
		return ""
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

	rows := buildRows(m.shown)
	h := m.listHeight()
	n := 0
	for i := m.offset; i < len(rows) && n < h; i++ {
		r := rows[i]
		if r.entry < 0 {
			b.WriteString(styleHeader.Render(fit(" "+r.header, m.width)) + "\n")
		} else {
			b.WriteString(m.renderEntry(m.shown[r.entry], r.entry == m.cursor) + "\n")
		}
		n++
	}
	for ; n < h; n++ {
		b.WriteString("\n")
	}
	b.WriteString(styleDim.Render(strings.Repeat("─", max(m.width, 1))) + "\n")
	b.WriteString(m.detailLine() + "\n")
	b.WriteString(styleDim.Render(fit(" "+helpText, m.width)))
	return b.String()
}

// Fila: marca(2) + número(3) + indicador(1) + 2 + ubicación + 2 + título.
func (m Model) renderEntry(e entries.Entry, selected bool) string {
	mark := "  "
	if selected {
		mark = styleTitle.Render("› ")
	}
	titleW := m.width - (2 + 3 + 1 + 2 + folderWidth + 2)
	text := fit(location(e), folderWidth) + "  " + fit(e.Title, titleW)
	if selected {
		text = styleSel.Render(text)
	}
	line := mark + fmt.Sprintf("%2d ", e.Num) + indicator(e.State, m.frame) + "  " + text
	// El prefijo es de ancho fijo: en un popup angosto (o con números de 3
	// dígitos) se pasaría del ancho y la terminal partiría la línea.
	return ansi.Truncate(line, m.width, "")
}

func (m Model) detailLine() string {
	if m.err != nil {
		return styleErr.Render(fit(" error: "+m.err.Error(), m.width))
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
	if e.State != state.Unknown && !e.Since.IsZero() {
		parts = append(parts, humanSince(m.d.Now().Sub(e.Since)))
	}
	return styleDim.Render(fit(" "+strings.Join(parts, " · "), m.width))
}
