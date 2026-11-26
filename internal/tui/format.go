package tui

import (
	"fmt"
	"math"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"github.com/jstrah00/ccmcli/internal/entries"
	"github.com/jstrah00/ccmcli/internal/state"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Estados: rojo/amarillo/verde semánticos; grises cálidos a juego con la
// paleta de Claude Code.
var (
	colWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true)
	colWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	colDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Bold(true)
	colIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a8a095"))
	colUnknown = lipgloss.NewStyle().Foreground(lipgloss.Color("#7a7268"))
)

const remoteGlyph = "◉"

// remoteFrames: el indicador de remote-control "respira" entre un azul
// apagado y uno vivo; un ciclo dura len(remoteFrames) * spinEvery.
var remoteFrames = breathe([3]float64{40, 80, 110}, [3]float64{125, 207, 255}, 24)

func breathe(lo, hi [3]float64, n int) []lipgloss.Style {
	out := make([]lipgloss.Style, n)
	for i := range out {
		t := (1 - math.Cos(2*math.Pi*float64(i)/float64(n))) / 2
		var c [3]int
		for k := range c {
			c[k] = int(math.Round(lo[k] + (hi[k]-lo[k])*t))
		}
		out[i] = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", c[0], c[1], c[2])))
	}
	return out
}

func remoteStyle(frame int) lipgloss.Style { return remoteFrames[frame%len(remoteFrames)] }

func indicator(st state.State, frame int) string {
	style, glyph := indicatorParts(st, frame)
	return style.Render(glyph)
}

func indicatorParts(st state.State, frame int) (lipgloss.Style, string) {
	switch st {
	case state.Waiting:
		return colWaiting, "●"
	case state.Working:
		return colWorking, spinnerFrames[frame%len(spinnerFrames)]
	case state.Done:
		return colDone, "✔"
	case state.Idle:
		return colIdle, "○"
	default:
		return colUnknown, "?"
	}
}

func stateLabel(st state.State) string {
	switch st {
	case state.Waiting:
		return "waiting for input"
	case state.Working:
		return "working"
	case state.Done:
		return "finished"
	case state.Idle:
		return "idle"
	default:
		return "unknown"
	}
}

func humanSince(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// location dice (salvo que tenga alias) dónde está la entrada dentro de su sesión: el índice de la
// window, su nombre si se lo pusiste (no el automático, que es el comando) y
// la carpeta si no es la misma que el encabezado de sesión.
func location(e entries.Entry) string {
	if e.Alias != "" {
		return e.Alias
	}
	loc := fmt.Sprintf(":%d", e.Pane.Window)
	if n := e.Pane.WindowName; n != "" && n != e.Pane.Command {
		loc += " " + n
	}
	if e.Folder != e.Pane.Session {
		loc += " " + e.Folder
	}
	return loc
}

// fit deja s en exactamente w columnas: trunca con … o rellena con espacios.
func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	return runewidth.FillRight(runewidth.Truncate(s, w, "…"), w)
}
