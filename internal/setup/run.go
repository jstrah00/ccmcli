package setup

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Options struct {
	Bin      string
	Settings string
	TmuxConf string
	Key      string
	DryRun   bool
	// Source recarga tmux.conf en el server que esté corriendo; opcional.
	Source func(path string) error
}

var strayRe = regexp.MustCompile(`ccm seen|/ccm["']?\s*$`)

// StrayLines devuelve los números de línea (desde 1) de una instalación a
// mano de ccm fuera del bloque, que quedaría duplicada.
func StrayLines(conf string) []int {
	var out []int
	inside := false
	for i, l := range strings.Split(conf, "\n") {
		switch strings.TrimSpace(l) {
		case blockStart:
			inside = true
			continue
		case blockEnd:
			inside = false
			continue
		}
		if !inside && !strings.HasPrefix(strings.TrimSpace(l), "#") && strayRe.MatchString(l) {
			out = append(out, i+1)
		}
	}
	return out
}

func readOptional(p string) ([]byte, bool, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	return b, err == nil, err
}

// write sigue symlinks a propósito (dotfiles con stow): un rename atómico
// reemplazaría el symlink por un archivo.
func write(p string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func Run(o Options, out io.Writer) error {
	if !ValidKey(o.Key) {
		return fmt.Errorf("invalid tmux key %q", o.Key)
	}
	if strings.Contains(o.Bin, "go-build") {
		return fmt.Errorf("%s is a temporary `go run` binary: install ccm with `go install github.com/jstrah00/ccmcli/cmd/ccm@latest` and run that one", o.Bin)
	}
	verb := "updated"
	if o.DryRun {
		verb = "would update"
	}

	settings, existed, err := readOptional(o.Settings)
	if err != nil {
		return err
	}
	merged, changed, err := MergeHooks(settings, o.Bin)
	if err != nil {
		return fmt.Errorf("%s: %w (fix it by hand and run ccm setup again)", o.Settings, err)
	}
	if !changed {
		fmt.Fprintf(out, "· Claude Code hooks already set up in %s\n", o.Settings)
	} else {
		if !o.DryRun {
			if existed {
				if err := write(o.Settings+".bak-ccm", settings); err != nil {
					return err
				}
			}
			if err := write(o.Settings, merged); err != nil {
				return err
			}
		}
		fmt.Fprintf(out, "✔ Claude Code hooks %s in %s\n", verb, o.Settings)
		for _, e := range Events {
			fmt.Fprintf(out, "    %-17s %s\n", e.Name, Command(o.Bin, e.Arg))
		}
		if existed && !o.DryRun {
			fmt.Fprintf(out, "    backup: %s.bak-ccm\n", o.Settings)
		}
	}

	conf, _, err := readOptional(o.TmuxConf)
	if err != nil {
		return err
	}
	block := TmuxBlock(o.Bin, o.Key)
	newConf, tchanged := MergeTmux(string(conf), block)
	if !tchanged {
		fmt.Fprintf(out, "· tmux already set up in %s (%s opens ccm)\n", o.TmuxConf, o.Key)
	} else {
		if !o.DryRun {
			if err := write(o.TmuxConf, []byte(newConf)); err != nil {
				return err
			}
		}
		fmt.Fprintf(out, "✔ tmux %s in %s: %s opens ccm\n", verb, o.TmuxConf, o.Key)
		for _, l := range strings.Split(strings.TrimSpace(block), "\n") {
			fmt.Fprintf(out, "    %s\n", l)
		}
		if !o.DryRun {
			if o.Source != nil && o.Source(o.TmuxConf) == nil {
				fmt.Fprintln(out, "    reloaded in the running tmux server")
			} else {
				fmt.Fprintf(out, "    reload it with: tmux source-file %s\n", o.TmuxConf)
			}
		}
	}
	if s := StrayLines(newConf); len(s) > 0 {
		fmt.Fprintf(out, "! %s has ccm lines outside the ccm block (lines %v): remove them to avoid running ccm twice\n", o.TmuxConf, s)
	}
	if !o.DryRun {
		fmt.Fprintf(out, "\nNext: restart your open Claude Code sessions, then press %s inside tmux.\n", o.Key)
	}
	return nil
}
