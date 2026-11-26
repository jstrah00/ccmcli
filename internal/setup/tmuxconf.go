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
// en cada source-file. bin va entre comillas de shell dentro de comillas
// dobles de tmux; Run rechaza las rutas que romperían alguna de las dos.
func TmuxBlock(bin, key string) string {
	q := ShellQuote(bin)
	return fmt.Sprintf(`%s
bind-key -n %s display-popup -E -h "60%%" -w "80%%" "%s"
set-hook -g session-window-changed[42] "run-shell -b \"%s seen\""
set-hook -g client-session-changed[42] "run-shell -b \"%s seen\""
%s
`, BlockStart, key, q, q, q, blockEnd)
}

var unsafeBin = regexp.MustCompile("[\"'$#\\\\`%\n]")

// SafeBin: la ruta del binario va dentro de comillas de tmux y de shell.
func SafeBin(bin string) bool { return !unsafeBin.MatchString(bin) }

// MergeTmux agrega el bloque de ccm al final, o reemplaza el que ya esté.
// Con marcadores rotos (editados a mano) no toca nada: adivinar dónde termina
// el bloque puede borrar config del usuario.
func MergeTmux(conf, block string) (string, bool, error) {
	ns, ne := strings.Count(conf, BlockStart), strings.Count(conf, blockEnd)
	i, j := strings.Index(conf, BlockStart), strings.Index(conf, blockEnd)
	if ns > 1 || ne > 1 || ns != ne || (ns == 1 && j < i) {
		return "", false, fmt.Errorf("the %q / %q markers are broken: fix them by hand (keep exactly one of each, in order) or remove both", BlockStart, blockEnd)
	}
	var out string
	if ns == 1 {
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
	return out, out != conf, nil
}
