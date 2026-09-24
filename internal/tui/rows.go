package tui

import "ccm/internal/entries"

// row es una línea de la lista: un encabezado de sesión (entry == -1) o una
// entrada (entry = índice en la lista mostrada).
type row struct {
	header string
	entry  int
}

// buildRows emite un encabezado cada vez que cambia la sesión respecto de la
// fila anterior; una sesión se repite si sus entradas no son contiguas.
func buildRows(es []entries.Entry) []row {
	var rows []row
	for i, e := range es {
		if i == 0 || e.Pane.Session != es[i-1].Pane.Session {
			rows = append(rows, row{header: e.Pane.Session, entry: -1})
		}
		rows = append(rows, row{entry: i})
	}
	return rows
}
