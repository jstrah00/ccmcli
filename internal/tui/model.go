// Package tui es el popup de ccm.
package tui

import (
	"sort"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sahilm/fuzzy"

	"ccm/internal/entries"
	"ccm/internal/order"
	"ccm/internal/tmux"
)

type Deps struct {
	Load      func() ([]entries.Entry, error)
	Order     *order.Store
	SaveOrder func() error
	Switch    func(tmux.PaneInfo) error
	Now       func() time.Time
}

type mode int

const (
	normal mode = iota
	filtering
)

const (
	refreshEvery = time.Second
	spinEvery    = 100 * time.Millisecond
	seqTimeout   = time.Second            // dd, gg
	digitTimeout = 300 * time.Millisecond // segundo dígito
)

type (
	tickMsg    struct{}
	spinMsg    struct{}
	timeoutMsg struct{ seq int }
)

type Model struct {
	d        Deps
	live     []entries.Entry // como vienen de Load
	list     []entries.Entry // visibles, ordenadas y numeradas
	shown    []entries.Entry // list filtrada
	cursor   int             // índice en shown
	mode     mode
	filter   string
	pending  string // "d" o "g" esperando la segunda tecla
	digit    string // primer dígito esperando el segundo
	seq      int    // invalida timeouts viejos
	register string // clave cortada con dd
	frame    int
	width    int
	height   int
	offset   int // primera fila visible
	err      error
	quitting bool
}

func New(d Deps) Model {
	m := Model{d: d, width: 80, height: 24}
	m.reload()
	return m
}

func (m Model) Init() tea.Cmd { return tea.Batch(tick(), spin()) }

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

func spin() tea.Cmd {
	return tea.Tick(spinEvery, func(time.Time) tea.Msg { return spinMsg{} })
}

func (m *Model) after(d time.Duration) tea.Cmd {
	m.seq++
	s := m.seq
	return tea.Tick(d, func(time.Time) tea.Msg { return timeoutMsg{seq: s} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	next, cmd := m.update(msg)
	next.scroll()
	return next, cmd
}

func (m Model) update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
	case tickMsg:
		m.reload()
		return m, tick()
	case spinMsg:
		m.frame++
		return m, spin()
	case timeoutMsg:
		if msg.seq != m.seq {
			return m, nil
		}
		m.pending = ""
		if d := m.digit; d != "" {
			m.digit = ""
			return m.jump(d)
		}
	case tea.KeyMsg:
		if m.mode == filtering {
			return m.filterKey(msg)
		}
		return m.normalKey(msg)
	}
	return m, nil
}

func (m Model) normalKey(k tea.KeyMsg) (Model, tea.Cmd) {
	key := k.String()
	if d := m.digit; d != "" {
		m.digit = ""
		if isDigit(key) {
			return m.jump(d + key)
		}
		return m.jump(d)
	}
	if p := m.pending; p != "" {
		m.pending = ""
		switch p + key {
		case "dd":
			m.hide()
			return m, nil
		case "gg":
			m.cursor = 0
			return m, nil
		}
		// cualquier otra tecla cancela la secuencia y se procesa normal
	}
	switch key {
	case "q", "ctrl+c":
		return m.quit()
	case "esc":
		if m.filter != "" {
			m.setFilter("")
			return m, nil
		}
		return m.quit()
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "G":
		m.cursor = len(m.shown) - 1
		m.clamp()
	case "d", "g":
		m.pending = key
		return m, m.after(seqTimeout)
	case "p":
		m.paste(true)
	case "P":
		m.paste(false)
	case "U":
		m.restoreAll()
	case "/":
		m.mode = filtering
	case "enter":
		return m.open(m.selected())
	default:
		if isDigit(key) && key != "0" {
			n, _ := strconv.Atoi(key)
			if len(m.list) >= 10 && n*10 <= len(m.list) {
				m.digit = key
				return m, m.after(digitTimeout)
			}
			return m.jump(key)
		}
	}
	return m, nil
}

func (m Model) filterKey(k tea.KeyMsg) (Model, tea.Cmd) {
	switch k.Type {
	case tea.KeyCtrlC:
		return m.quit()
	case tea.KeyEsc:
		m.mode = normal
		m.setFilter("")
	case tea.KeyEnter:
		m.mode = normal
	case tea.KeyBackspace:
		if r := []rune(m.filter); len(r) > 0 {
			m.setFilter(string(r[:len(r)-1]))
		}
	case tea.KeyCtrlN, tea.KeyDown:
		m.move(1)
	case tea.KeyCtrlP, tea.KeyUp:
		m.move(-1)
	case tea.KeySpace:
		m.setFilter(m.filter + " ")
	case tea.KeyRunes:
		m.setFilter(m.filter + string(k.Runes))
	}
	return m, nil
}

func isDigit(s string) bool { return len(s) == 1 && s[0] >= '0' && s[0] <= '9' }

func (m Model) quit() (Model, tea.Cmd) {
	m.quitting = true
	return m, tea.Quit
}

// jump usa siempre el número global, no la posición filtrada.
func (m Model) jump(num string) (Model, tea.Cmd) {
	n, err := strconv.Atoi(num)
	if err != nil {
		return m, nil
	}
	for i := range m.list {
		if m.list[i].Num == n {
			return m.open(&m.list[i])
		}
	}
	return m, nil
}

func (m Model) open(e *entries.Entry) (Model, tea.Cmd) {
	if e == nil {
		return m, nil
	}
	if err := m.d.Switch(e.Pane); err != nil {
		m.err = err
		return m, nil
	}
	return m.quit()
}

func (m *Model) reload() {
	live, err := m.d.Load()
	if err != nil {
		m.err = err // se mantiene la última lista buena
		return
	}
	m.err = nil
	m.live = live
	m.relist()
}

func (m *Model) relist() {
	key := m.currentKey()
	m.list = entries.Arrange(m.live, m.d.Order.Visible(entries.Keys(m.live)))
	m.applyFilter()
	m.placeCursor(key)
}

// applyFilter mantiene el orden de la lista (no el del score) para que los
// encabezados sigan teniendo sentido.
func (m *Model) applyFilter() {
	if m.filter == "" {
		m.shown = m.list
		return
	}
	src := make([]string, len(m.list))
	for i, e := range m.list {
		src[i] = e.Pane.Session + " " + e.Folder + " " + e.Title
	}
	matches := fuzzy.Find(m.filter, src)
	sort.Slice(matches, func(i, j int) bool { return matches[i].Index < matches[j].Index })
	m.shown = make([]entries.Entry, 0, len(matches))
	for _, mt := range matches {
		m.shown = append(m.shown, m.list[mt.Index])
	}
}

func (m *Model) setFilter(f string) {
	key := m.currentKey()
	m.filter = f
	m.applyFilter()
	m.placeCursor(key)
}

// placeCursor vuelve a la misma entrada; si ya no está, el índice queda igual
// (la siguiente ocupa su lugar) o baja a la última.
func (m *Model) placeCursor(key string) {
	for i := range m.shown {
		if m.shown[i].Key == key {
			m.cursor = i
			return
		}
	}
	m.clamp()
}

func (m *Model) clamp() {
	if m.cursor >= len(m.shown) {
		m.cursor = len(m.shown) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) move(d int) {
	m.cursor += d
	m.clamp()
}

func (m *Model) selected() *entries.Entry {
	if m.cursor >= 0 && m.cursor < len(m.shown) {
		return &m.shown[m.cursor]
	}
	return nil
}

func (m *Model) currentKey() string {
	if e := m.selected(); e != nil {
		return e.Key
	}
	return ""
}

func (m *Model) hide() {
	e := m.selected()
	if e == nil {
		return
	}
	key := e.Key
	m.d.Order.Hide(entries.Keys(m.live), key)
	m.register = key
	m.save()
	m.relist()
}

func (m *Model) paste(below bool) {
	if m.register == "" {
		return
	}
	key := m.register
	m.register = ""
	m.d.Order.Paste(entries.Keys(m.live), key, m.currentKey(), below)
	m.save()
	m.relist()
	m.placeCursor(key)
}

func (m *Model) restoreAll() {
	m.d.Order.RestoreAll(entries.Keys(m.live))
	m.save()
	m.relist()
}

func (m *Model) save() {
	if err := m.d.SaveOrder(); err != nil {
		m.err = err
	}
}

// listHeight: alto total menos título, separador, detalle y ayuda.
func (m *Model) listHeight() int {
	if h := m.height - 4; h > 0 {
		return h
	}
	return 1
}

// scroll ajusta offset para que el cursor (y, si se puede, su encabezado)
// quede visible.
func (m *Model) scroll() {
	rows := buildRows(m.shown)
	cur := -1
	for i, r := range rows {
		if r.entry == m.cursor {
			cur = i
			break
		}
	}
	if cur < 0 {
		m.offset = 0
		return
	}
	top := cur
	if cur > 0 && rows[cur-1].entry < 0 {
		top = cur - 1
	}
	if top < m.offset {
		m.offset = top
	}
	if h := m.listHeight(); cur >= m.offset+h {
		m.offset = cur - h + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m Model) View() string { return "" }
