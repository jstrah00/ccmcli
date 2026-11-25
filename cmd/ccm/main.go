// ccm: selector de sesiones de Claude Code dentro de tmux.
//
//	ccm               abre la TUI (pensado para display-popup)
//	ccm hook <evento> lo llaman los hooks de Claude Code
//	ccm seen          lo llaman los hooks de tmux al cambiar de window
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ccm/internal/closed"
	"ccm/internal/entries"
	"ccm/internal/hook"
	"ccm/internal/order"
	"ccm/internal/paths"
	"ccm/internal/proc"
	"ccm/internal/remote"
	"ccm/internal/state"
	"ccm/internal/tmux"
	"ccm/internal/transcript"
	"ccm/internal/tui"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return runTUI(stderr)
	}
	switch args[0] {
	case "hook":
		if len(args) < 2 {
			logf("hook sin evento")
			return 0
		}
		runHook(args[1], stdin)
		return 0
	case "seen":
		runSeen()
		return 0
	}
	fmt.Fprintln(stderr, "uso: ccm [hook <evento> | seen]")
	return 2
}

// logf nunca escribe a stdout/stderr: Claude Code lee el stdout del hook.
func logf(format string, a ...any) {
	p := paths.LogPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, a...))
}

func runHook(event string, stdin io.Reader) {
	defer func() {
		if r := recover(); r != nil {
			logf("hook %s: panic: %v", event, r)
		}
	}()
	d := hook.Deps{
		Store:  state.Store{Dir: paths.PanesDir()},
		PaneID: os.Getenv("TMUX_PANE"),
		Now:    time.Now,
		PaneActive: func(id string) (bool, error) {
			bin, err := tmux.Find()
			if err != nil {
				return false, err
			}
			return tmux.Exec{Bin: bin}.PaneActive(id)
		},
		BgHost: func() (proc.BgHost, bool) { return proc.FindBgHost(os.Getpid(), proc.PS) },
		Closed: closed.Store{Path: paths.ClosedPath()},
		PaneLoc: func(id string) (tmux.PaneInfo, error) {
			bin, err := tmux.Find()
			if err != nil {
				return tmux.PaneInfo{}, err
			}
			return tmux.Exec{Bin: bin}.PaneLoc(id)
		},
		Labels: func(key string) (string, string) {
			ord, _ := order.Load(paths.OrderPath())
			return ord.Alias(key), ord.Status(key)
		},
	}
	if err := hook.Run(event, stdin, d); err != nil {
		logf("hook %s: %v", event, err)
	}
}

func runSeen() {
	bin, err := tmux.Find()
	if err != nil {
		logf("seen: %v", err)
		return
	}
	if err := hook.Seen(tmux.Exec{Bin: bin}, state.Store{Dir: paths.PanesDir()}, time.Now()); err != nil {
		logf("seen: %v", err)
	}
}

func runTUI(stderr io.Writer) int {
	if os.Getenv("TMUX") == "" {
		fmt.Fprintln(stderr, "ccm: hay que correrlo dentro de tmux")
		return 1
	}
	bin, err := tmux.Find()
	if err != nil {
		fmt.Fprintln(stderr, "ccm:", err)
		return 1
	}
	tm := tmux.Exec{Bin: bin}
	store := state.Store{Dir: paths.PanesDir()}
	titles := transcript.NewCache(paths.TitlesDir())
	closedStore := closed.Store{Path: paths.ClosedPath()}
	ord, err := order.Load(paths.OrderPath())
	if err != nil {
		logf("%v", err) // orden por defecto; se reescribe en el próximo cambio
	}

	purged := false
	load := func() ([]entries.Entry, error) {
		panes, err := tm.ListPanes()
		if err != nil {
			return nil, err
		}
		states, err := store.All()
		if err != nil {
			return nil, err
		}
		if !purged {
			entries.Purge(store, closedStore, panes, states, time.Now())
			purged = true
		}
		return entries.Collect(panes, states, titles.Get, remote.Active(remote.Dir())), nil
	}

	cur, err := tm.CurrentPane()
	if err != nil {
		logf("%v", err)
	}

	m := tui.New(tui.Deps{
		Load:      load,
		Order:     ord,
		SaveOrder: func() error { return ord.Save(paths.OrderPath()) },
		Switch:    tm.Switch,
		Now:       time.Now,
		Current:   cur,
		Closed:    closedStore.List,
		Titles:    titles.Get,
		Resume:    func(c closed.Session) (tmux.PaneInfo, error) { return closed.Resume(c, tm) },
	})
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(stderr, "ccm:", err)
		return 1
	}
	return 0
}
