package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"ccm/internal/entries"
	"ccm/internal/state"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Paleta tokyonight, la misma del status bar de tmux.
var (
	colWaiting = lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e")).Bold(true)
	colWorking = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	colDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a")).Bold(true)
	colIdle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))
	colUnknown = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
)

func indicator(st state.State, frame int) string {
	switch st {
	case state.Waiting:
		return colWaiting.Render("●")
	case state.Working:
		return colWorking.Render(spinnerFrames[frame%len(spinnerFrames)])
	case state.Done:
		return colDone.Render("✓")
	case state.Idle:
		return colIdle.Render("○")
	default:
		return colUnknown.Render("?")
	}
}

func stateLabel(st state.State) string {
	switch st {
	case state.Waiting:
		return "esperando input"
	case state.Working:
		return "trabajando"
	case state.Done:
		return "terminó"
	case state.Idle:
		return "idle"
	default:
		return "desconocido"
	}
}

func humanSince(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("hace %ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("hace %dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("hace %dh", int(d.Hours()))
	default:
		return fmt.Sprintf("hace %dd", int(d.Hours()/24))
	}
}

// location dice dónde está la entrada dentro de su sesión: el índice de la
// window, su nombre si se lo pusiste (no el automático, que es el comando) y
// la carpeta si no es la misma que el encabezado de sesión.
func location(e entries.Entry) string {
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
