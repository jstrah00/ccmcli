package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	BlockStart = "# >>> ccm >>>"
	blockEnd   = "# <<< ccm <<<"
)

// DefaultKey abre el popup sin prefijo de tmux.
const DefaultKey = "C-g"

// TmuxConfPath prefiere el tmux.conf de XDG si existe, como hace tmux ≥ 3.1.
func TmuxConfPath() string {
	home, _ := os.UserHomeDir()
	xdg := os.Getenv("XDG_CONFIG_HOME")
	if xdg == "" {
		xdg = filepath.Join(home, ".config")
	}
	if p := filepath.Join(xdg, "tmux", "tmux.conf"); exists(p) {
		return p
	}
	return filepath.Join(home, ".tmux.conf")
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

var keyRe = regexp.MustCompile(`^[A-Za-z0-9_+\-]+$`)

func ValidKey(k string) bool { return keyRe.MatchString(k) }

// TmuxBlock usa el índice 42 en los hooks de tmux: con -g a secas pisaría
// los hooks que el usuario ya tenga en esos eventos, y con -a se duplicaría
// en cada source-file.
func TmuxBlock(bin, key string) string {
	return fmt.Sprintf(`%s
bind-key -n %s display-popup -E -h "60%%" -w "80%%" "%s"
set-hook -g session-window-changed[42] 'run-shell -b "%s seen"'
set-hook -g client-session-changed[42] 'run-shell -b "%s seen"'
%s
`, BlockStart, key, bin, bin, bin, blockEnd)
}

// MergeTmux agrega el bloque de ccm al final, o reemplaza el que ya esté.
func MergeTmux(conf, block string) (string, bool) {
	i := strings.Index(conf, BlockStart)
	j := strings.Index(conf, blockEnd)
	var out string
	if i >= 0 && j > i {
		end := j + len(blockEnd)
		if end < len(conf) && conf[end] == '\n' {
			end++
		}
		out = conf[:i] + block + conf[end:]
	} else {
		out = conf
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if out != "" {
			out += "\n"
		}
		out += block
	}
	return out, out != conf
}
