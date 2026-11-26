package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"

	"github.com/jstrah00/ccmcli/internal/closed"
	"github.com/jstrah00/ccmcli/internal/order"
)

var closedHelp = []keyHelp{{"↑/↓", "move"}, {"⏎", "resume"}, {"esc", "back"}}

const ageWidth = 9

// closedRow es una sesión cerrada lista para mostrar. hay es el texto sobre
// el que busca el fuzzy: las tres columnas visibles primero, en el orden en
// que se dibujan, para poder resaltar las coincidencias por offset.
type closedRow struct {
	s       closed.Session
	loc     string
	title   string
	hay     string
	matched map[int]bool // offsets en bytes dentro de hay
}

func newClosedRow(s closed.Session, title string) closedRow {
	r := closedRow{s: s, loc: s.Alias, title: title}
	if r.loc == "" {
		r.loc = filepath.Base(s.Cwd)
	}
	if r.title == "" {
		r.title = s.LastPrompt
	}
	r.hay = strings.Join([]string{s.TmuxSession, r.loc, r.title, s.Status, s.LastPrompt}, " ")
	return r
}

func (r closedRow) locOffset() int   { return len(r.s.TmuxSession) + 1 }
func (r closedRow) titleOffset() int { return r.locOffset() + len(r.loc) + 1 }

// openClosed carga la lista una sola vez: mientras buscás no se mueve.
func (m *Model) openClosed() {
	if m.d.Closed == nil {
		return
	}
	m.closedAll = m.closedAll[:0]
	for _, s := range m.d.Closed() {
		title := ""
		if m.d.Titles != nil {
			title = m.d.Titles(s.SessionID, s.TranscriptPath).Row()
		}
		m.closedAll = append(m.closedAll, newClosedRow(s, title))
	}
	m.mode = closedView
	m.err = nil
	m.setQuery("")
}

// setQuery: sin query, orden de cierre; con query, el del score.
func (m *Model) setQuery(q string) {
	m.query = q
	m.ccursor, m.coffset = 0, 0
	if q == "" {
		m.closedShown = m.closedAll
		return
	}
	src := make([]string, len(m.closedAll))
	for i, r := range m.closedAll {
		src[i] = r.hay
	}
	matches := fuzzy.Find(q, src)
	m.closedShown = make([]closedRow, 0, len(matches))
	for _, mt := range matches {
		r := m.closedAll[mt.Index]
		r.matched = make(map[int]bool, len(mt.MatchedIndexes))
		for _, i := range mt.MatchedIndexes {
			r.matched[i] = true
		}
		m.closedShown = append(m.closedShown, r)
	}
}

func (m *Model) closedMove(d int) {
	m.ccursor = max(0, min(m.ccursor+d, len(m.closedShown)-1))
}

func (m *Model) closedScroll() {
	h := m.listHeight()
	if m.ccursor < m.coffset {
		m.coffset = m.ccursor
	}
	if m.ccursor >= m.coffset+h {
		m.coffset = m.ccursor - h + 1
	}
	m.coffset = max(m.coffset, 0)
}

func (m Model) closedKey(k tea.KeyMsg) (Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyCtrlC:
		return m.quit()
	case tea.KeyEsc:
		m.err = nil
		if m.query != "" {
			m.setQuery("")
		} else {
			m.mode = normal
		}
	case tea.KeyEnter:
		return m.resume()
	case tea.KeyBackspace:
		if r := []rune(m.query); len(r) > 0 {
			m.setQuery(string(r[:len(r)-1]))
		}
	case tea.KeyDown, tea.KeyCtrlN, tea.KeyCtrlJ:
		m.closedMove(1)
	case tea.KeyUp, tea.KeyCtrlP, tea.KeyCtrlK:
		m.closedMove(-1)
	case tea.KeySpace:
		m.setQuery(m.query + " ")
	case tea.KeyRunes:
		m.setQuery(m.query + string(k.Runes))
	}
	return m, nil
}

func (m Model) resume() (Model, tea.Cmd) {
	if m.ccursor >= len(m.closedShown) || m.d.Resume == nil {
		return m, nil
	}
	s := m.closedShown[m.ccursor].s
	p, err := m.d.Resume(s)
	if err != nil {
		m.err = err
		return m, nil
	}
	if s.Alias != "" || s.Status != "" {
		key := order.Key(p.Session, p.Window, p.Pane, p.Path)
		m.d.Order.SetAlias(key, s.Alias)
		m.d.Order.SetStatus(key, s.Status)
		m.save()
	}
	return m.quit()
}

func (m Model) closedStatus() string {
	n := fmt.Sprintf("%d", len(m.closedAll))
	if m.query != "" {
		n = fmt.Sprintf("%d/%d", len(m.closedShown), len(m.closedAll))
	}
	return styleTitle.Render("Closed") + styleDim.Render(" · "+n)
}

func (m Model) closedViewRender() string {
	var b strings.Builder
	for _, l := range m.header() {
		b.WriteString(l + "\n")
	}
	if m.query == "" {
		b.WriteString(styleDim.Render(fit(" > type to search", m.width)) + "\n")
	} else {
		b.WriteString(styleSel.Render(fit(" > "+m.query+"▏", m.width)) + "\n")
	}

	h := m.listHeight()
	n := 0
	switch {
	case len(m.closedAll) == 0:
		b.WriteString(styleDim.Render(fit("  no closed sessions yet", m.width)) + "\n")
		n++
	case len(m.closedShown) == 0:
		b.WriteString(styleDim.Render(fit("  no results", m.width)) + "\n")
		n++
	default:
		sessW, locW := m.closedWidths()
		for i := m.coffset; i < len(m.closedShown) && n < h; i++ {
			b.WriteString(m.renderClosed(m.closedShown[i], i == m.ccursor, sessW, locW) + "\n")
			n++
		}
	}
	for ; n < h; n++ {
		b.WriteString("\n")
	}
	b.WriteString(styleDim.Render(strings.Repeat("─", max(m.width, 1))) + "\n")
	b.WriteString(m.closedDetail() + "\n")
	b.WriteString(m.helpLine())
	return b.String()
}

// closedWidths mira la lista completa para que las columnas no bailen al
// filtrar.
func (m Model) closedWidths() (sessW, locW int) {
	for _, r := range m.closedAll {
		sessW = max(sessW, runewidth.StringWidth(r.s.TmuxSession))
		locW = max(locW, runewidth.StringWidth(r.loc))
	}
	return min(sessW, max(m.width/5, 1)), min(locW, max(m.width/4, 1))
}

func (m Model) renderClosed(r closedRow, selected bool, sessW, locW int) string {
	paint := func(st lipgloss.Style, s string) string {
		if selected {
			st = st.Background(selBg)
		}
		return st.Render(s)
	}
	text, dim := lipgloss.NewStyle(), styleDim
	if selected {
		text, dim = styleSel, styleSel
	}
	mark := paint(lipgloss.NewStyle(), "  ")
	if selected {
		mark = paint(styleAccent, "▎ ")
	}
	cell := func(s string, off, w int, base lipgloss.Style) string {
		return highlight(s, off, w, r.matched, func(hit bool, s string) string {
			if hit {
				return paint(styleAccent, s)
			}
			return paint(base, s)
		})
	}
	age := humanSince(m.d.Now().Sub(r.s.ClosedAt))
	titleW := m.width - (2 + ageWidth + 1 + sessW + 2 + locW + 2)
	line := mark + paint(dim, fit(age, ageWidth)) + paint(text, " ") +
		cell(r.s.TmuxSession, 0, sessW, dim) + paint(text, "  ") +
		cell(r.loc, r.locOffset(), locW, text.Bold(true)) + paint(text, "  ") +
		cell(r.title, r.titleOffset(), titleW, text)
	line = ansi.Truncate(line, m.width, "")
	if pad := m.width - lipgloss.Width(line); pad > 0 {
		line += paint(lipgloss.NewStyle(), strings.Repeat(" ", pad))
	}
	return line
}

// highlight deja s en w columnas (como fit) pintando por tramos: hit dice si
// el tramo coincide con la búsqueda. off es dónde empieza s dentro del
// haystack; matched trae offsets en bytes.
func highlight(s string, off, w int, matched map[int]bool, paint func(hit bool, s string) string) string {
	if w <= 0 {
		return ""
	}
	cut := runewidth.Truncate(s, w, "…")
	var b strings.Builder
	flush := func(hit bool, run string) {
		if run != "" {
			b.WriteString(paint(hit, run))
		}
	}
	run, runHit := "", false
	for i, r := range cut {
		// El … del truncado no es parte de s aunque caiga en un offset marcado.
		orig, _ := utf8.DecodeRuneInString(s[min(i, len(s)):])
		hit := matched[off+i] && orig == r
		if hit != runHit {
			flush(runHit, run)
			run, runHit = "", hit
		}
		run += string(r)
	}
	flush(runHit, run)
	if pad := w - runewidth.StringWidth(cut); pad > 0 {
		b.WriteString(paint(false, strings.Repeat(" ", pad)))
	}
	return b.String()
}

func (m Model) closedDetail() string {
	if m.err != nil {
		return styleErr.Render(fit(" error: "+m.err.Error(), m.width))
	}
	if m.ccursor >= len(m.closedShown) {
		return ""
	}
	s := m.closedShown[m.ccursor].s
	line := styleDim.Render(" " + s.LastPrompt + " · closed " + humanSince(m.d.Now().Sub(s.ClosedAt)))
	if s.Status != "" {
		line = " " + styleStatus.Render(s.Status) + styleDim.Render(" ·") + line
	}
	return ansi.Truncate(line, m.width, "…")
}
