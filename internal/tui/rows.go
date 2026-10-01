package tui

import "ccm/internal/entries"

const (
	rowHeader = -1
	rowSpacer = -2
)

// row es una línea de la lista: un encabezado de sesión (entry == rowHeader,
// con count entradas), un espacio entre grupos (rowSpacer) o una entrada
// (entry = índice en la lista mostrada).
type row struct {
	header string
	count  int
	entry  int
}

// rows deja una línea en blanco entre entradas si así entra todo en pantalla;
// si no, prioriza mostrar más entradas.
func (m Model) rows() []row {
	if rows := buildRows(m.shown, true); len(rows) <= m.listHeight() {
		return rows
	}
	return buildRows(m.shown, false)
}

// buildRows emite un encabezado cada vez que cambia la sesión respecto de la
// fila anterior; una sesión se repite si sus entradas no son contiguas. Con
// airy, también separa las entradas dentro de una sesión.
func buildRows(es []entries.Entry, airy bool) []row {
	var rows []row
	h := -1
	for i, e := range es {
		if i == 0 || e.Pane.Session != es[i-1].Pane.Session {
			if i > 0 {
				rows = append(rows, row{entry: rowSpacer})
			}
			rows = append(rows, row{header: e.Pane.Session, entry: rowHeader})
			h = len(rows) - 1
		} else if airy {
			rows = append(rows, row{entry: rowSpacer})
		}
		rows[h].count++
		rows = append(rows, row{entry: i})
	}
	return rows
}
