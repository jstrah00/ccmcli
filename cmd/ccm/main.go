// ccm: selector de sesiones de Claude Code dentro de tmux.
//
//	ccm               abre la TUI (pensado para display-popup)
//	ccm hook <evento> lo llaman los hooks de Claude Code
//	ccm seen          lo llaman los hooks de tmux al cambiar de window
//	ccm notify        manda un aviso (lo lanza el hook; --test para probar)
//	ccm setup         instala los hooks de Claude Code y el bloque de tmux
//	ccm doctor        revisa la instalación (--notify manda un aviso de prueba)
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/jstrah00/ccmcli/internal/closed"
	"github.com/jstrah00/ccmcli/internal/config"
	"github.com/jstrah00/ccmcli/internal/doctor"
	"github.com/jstrah00/ccmcli/internal/entries"
	"github.com/jstrah00/ccmcli/internal/hook"
	"github.com/jstrah00/ccmcli/internal/notify"
	"github.com/jstrah00/ccmcli/internal/order"
	"github.com/jstrah00/ccmcli/internal/paths"
	"github.com/jstrah00/ccmcli/internal/proc"
	"github.com/jstrah00/ccmcli/internal/remote"
	"github.com/jstrah00/ccmcli/internal/setup"
	"github.com/jstrah00/ccmcli/internal/state"
	"github.com/jstrah00/ccmcli/internal/tmux"
	"github.com/jstrah00/ccmcli/internal/transcript"
	"github.com/jstrah00/ccmcli/internal/tui"
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
			logf("hook without event")
			return 0
		}
		runHook(args[1], stdin)
		return 0
	case "seen":
		runSeen()
		return 0
	case "notify":
		return runNotify(args[1:], stdin, stderr)
	case "setup":
		return runSetup(args[1:], stdout, stderr)
	case "doctor":
		return runDoctor(args[1:], stdin, stdout, stderr)
	}
	fmt.Fprintln(stderr, "usage: ccm [hook <event> | seen | notify [--test] | setup | doctor]")
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
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		logf("config: %v", err)
	}
	if cfg.Notify.Banner || cfg.Notify.Sound {
		d.Notify = spawnNotify
		d.Socket = notify.SocketFromTMUX(os.Getenv("TMUX"))
		d.MinTurn = time.Duration(cfg.Notify.MinTurnSeconds) * time.Second
	}
	if err := hook.Run(event, stdin, d); err != nil {
		logf("hook %s: %v", event, err)
	}
}

// spawnNotify lanza "ccm notify" en su propia sesión y no lo espera: el aviso
// (sonido, esperar el click en Linux) no puede demorar a Claude Code. El
// request va por un pipe ya escrito y cerrado, porque el hook sale enseguida y
// no puede quedar una goroutine copiando stdin.
func spawnNotify(r notify.Request) {
	bin, err := os.Executable()
	if err != nil {
		logf("notify: %v", err)
		return
	}
	b, _ := json.Marshal(r)
	pr, pw, err := os.Pipe()
	if err != nil {
		logf("notify: %v", err)
		return
	}
	_, _ = pw.Write(b)
	pw.Close()
	defer pr.Close()
	cmd := exec.Command(bin, "notify")
	cmd.Stdin = pr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		logf("notify: %v", err)
		return
	}
	_ = cmd.Process.Release()
}

func runNotify(args []string, stdin io.Reader, stderr io.Writer) int {
	cfg, err := config.Load(paths.ConfigPath())
	if err != nil {
		logf("config: %v", err)
	}
	if len(args) > 0 && args[0] == "--test" {
		r := notify.Request{Kind: notify.Finished, PaneID: os.Getenv("TMUX_PANE"), Folder: "ccm", Body: "Notifications work. Click to jump to this pane."}
		if err := notify.Send(r, cfg.Notify); err != nil {
			fmt.Fprintln(stderr, "ccm:", err)
			return 1
		}
		return 0
	}
	var r notify.Request
	if err := json.NewDecoder(stdin).Decode(&r); err != nil {
		logf("notify: %v", err)
		return 0
	}
	if err := notify.Send(r, cfg.Notify); err != nil {
		logf("notify: %v", err)
	}
	return 0
}

func self() (string, error) {
	// Sin resolver symlinks: si ccm está linkeado (stow, brew), los hooks
	// deben apuntar al link, que sobrevive a los upgrades.
	return os.Executable()
}

func runSetup(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(stderr)
	o := setup.Options{}
	fs.BoolVar(&o.DryRun, "dry-run", false, "show what would change without writing")
	fs.StringVar(&o.Settings, "settings", setup.SettingsPath(), "Claude Code settings.json")
	fs.StringVar(&o.TmuxConf, "tmux-conf", setup.TmuxConfPath(), "tmux config file")
	fs.StringVar(&o.Key, "key", setup.DefaultKey, "tmux key (no prefix) that opens ccm")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	bin, err := self()
	if err != nil {
		fmt.Fprintln(stderr, "ccm:", err)
		return 1
	}
	o.Bin = bin
	o.Source = func(path string) error {
		t, err := tmux.Find()
		if err != nil {
			return err
		}
		return exec.Command(t, "source-file", path).Run()
	}
	if err := setup.Run(o, stdout); err != nil {
		fmt.Fprintln(stderr, "ccm:", err)
		return 1
	}
	return 0
}

func runDoctor(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	bin, err := self()
	if err != nil {
		fmt.Fprintln(stderr, "ccm:", err)
		return 1
	}
	cfg, cfgErr := config.Load(paths.ConfigPath())
	e := doctor.Env{
		SettingsPath: setup.SettingsPath(),
		TmuxConfPath: setup.TmuxConfPath(),
		Bin:          bin,
		GOOS:         runtime.GOOS,
		Bundle:       notify.Bundle(cfg.Notify.TerminalBundle, os.Getenv),
		ConfigErr:    cfgErr,
		Look: func(name string) string {
			return notify.Find(name, "/opt/homebrew/bin/"+name, "/usr/local/bin/"+name)
		},
	}
	e.Settings, _ = os.ReadFile(e.SettingsPath)
	if len(e.Settings) == 0 {
		e.Settings = []byte("{}")
	}
	conf, _ := os.ReadFile(e.TmuxConfPath)
	e.TmuxConf = string(conf)
	if t, err := tmux.Find(); err == nil {
		out, _ := exec.Command(t, "-V").Output()
		e.TmuxVersion = strings.TrimSpace(string(out))
	}
	report, ok := doctor.Print(doctor.Run(e))
	fmt.Fprint(stdout, report)
	if len(args) > 0 && args[0] == "--notify" {
		fmt.Fprintln(stdout, "sending a test notification…")
		if code := runNotify([]string{"--test"}, stdin, stderr); code != 0 {
			return code
		}
	}
	if !ok {
		return 1
	}
	return 0
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
		fmt.Fprintln(stderr, "ccm: must run inside tmux")
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
