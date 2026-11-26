package setup

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const bin = "/usr/local/bin/ccm"

func hooksOf(t *testing.T, b []byte) map[string][]struct {
	Matcher string `json:"matcher"`
	Hooks   []struct {
		Command string `json:"command"`
	} `json:"hooks"`
} {
	t.Helper()
	var s struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &s); err != nil {
		t.Fatalf("%v\n%s", err, b)
	}
	return s.Hooks
}

func commands(t *testing.T, b []byte, event string) []string {
	var out []string
	for _, g := range hooksOf(t, b)[event] {
		for _, h := range g.Hooks {
			out = append(out, g.Matcher+"|"+h.Command)
		}
	}
	return out
}

func TestMergeHooksEmpty(t *testing.T) {
	for _, in := range []string{"", "{}"} {
		out, changed, err := MergeHooks([]byte(in), bin)
		if err != nil || !changed {
			t.Fatalf("%q: changed=%v err=%v", in, changed, err)
		}
		n := 0
		for _, e := range Events {
			cs := commands(t, out, e.Name)
			want := e.Matcher + "|" + bin + " hook " + e.Arg
			if !strings.Contains(strings.Join(cs, "\n"), want) {
				t.Fatalf("falta %q en %s: %q", want, e.Name, cs)
			}
			n++
		}
		if n != 8 {
			t.Fatalf("eventos = %d", n)
		}
	}
}

func TestMergeHooksKeepsOthersAndUnknownKeys(t *testing.T) {
	in := `{"zeta":1,"model":"x","hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done && echo"}]}]},"alpha":{"b":2,"a":1}}`
	out, _, err := MergeHooks([]byte(in), bin)
	if err != nil {
		t.Fatal(err)
	}
	cs := commands(t, out, "Stop")
	if len(cs) != 2 || cs[0] != "|say done && echo" || cs[1] != "|"+bin+" hook stop" {
		t.Fatalf("Stop = %q", cs)
	}
	s := string(out)
	if !(strings.Index(s, `"zeta"`) < strings.Index(s, `"model"`) && strings.Index(s, `"model"`) < strings.Index(s, `"alpha"`)) ||
		strings.Index(s, `"b"`) > strings.Index(s, `"a"`) {
		t.Fatalf("cambió el orden de las claves:\n%s", s)
	}
	if strings.Contains(s, `\u0026`) {
		t.Fatal("escapó &")
	}
}

func TestMergeHooksIdempotent(t *testing.T) {
	once, _, _ := MergeHooks([]byte(`{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"x"}]}]}}`), bin)
	twice, changed, err := MergeHooks(once, bin)
	if err != nil || changed || string(once) != string(twice) {
		t.Fatalf("changed=%v err=%v\n%s\n%s", changed, err, once, twice)
	}
}

func TestMergeHooksReplacesOldBinary(t *testing.T) {
	in := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"$HOME/go/bin/ccm hook stop","timeout":5}]}],
"Notification":[{"matcher":"permission_prompt","hooks":[{"type":"command","command":"/old/ccm hook permission"},{"type":"command","command":"keep-me"}]}]}}`
	out, changed, err := MergeHooks([]byte(in), bin)
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	if cs := commands(t, out, "Stop"); len(cs) != 1 || cs[0] != "|"+bin+" hook stop" {
		t.Fatalf("Stop = %q", cs)
	}
	got := strings.Join(commands(t, out, "Notification"), "\n")
	if strings.Contains(got, "/old/ccm") || !strings.Contains(got, "permission_prompt|keep-me") {
		t.Fatalf("Notification = %q", got)
	}
}

func TestMergeHooksQuotesBinWithSpaces(t *testing.T) {
	out, _, _ := MergeHooks([]byte(`{}`), "/Users/a b/ccm")
	if cs := commands(t, out, "Stop"); cs[0] != "|'/Users/a b/ccm' hook stop" {
		t.Fatalf("got %q", cs)
	}
	again, changed, _ := MergeHooks(out, "/Users/a b/ccm")
	if changed || string(again) != string(out) {
		t.Fatal("no idempotente con espacios")
	}
}

func TestMergeHooksBrokenJSON(t *testing.T) {
	if _, _, err := MergeHooks([]byte(`{"hooks":`), bin); err == nil {
		t.Fatal("sin error con JSON roto")
	}
	if _, _, err := MergeHooks([]byte(`{"hooks":[]}`), bin); err == nil {
		t.Fatal("sin error con hooks que no es objeto")
	}
}

func TestTmuxBlock(t *testing.T) {
	b := TmuxBlock(bin, "C-g")
	for _, want := range []string{
		"# >>> ccm >>>", "# <<< ccm <<<",
		`bind-key -n C-g display-popup -E -h "60%" -w "80%" "` + bin + `"`,
		`set-hook -g session-window-changed[42] "run-shell -b \"` + bin + ` seen\""`,
		`set-hook -g client-session-changed[42] "run-shell -b \"` + bin + ` seen\""`,
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("falta %q en:\n%s", want, b)
		}
	}
}

func TestMergeTmux(t *testing.T) {
	block := TmuxBlock(bin, "C-g")
	conf := "set -g mouse on\n"
	out, changed, _ := MergeTmux(conf, block)
	if !changed || !strings.HasPrefix(out, conf) || !strings.Contains(out, block) {
		t.Fatalf("append:\n%s", out)
	}
	if again, changed, _ := MergeTmux(out, block); changed || again != out {
		t.Fatal("no idempotente")
	}
	newBlock := TmuxBlock(bin, "M-c")
	repl, changed, _ := MergeTmux(out+"set -g x y\n", newBlock)
	if !changed || strings.Contains(repl, "C-g") || !strings.Contains(repl, "M-c") ||
		!strings.HasSuffix(repl, "set -g x y\n") || strings.Count(repl, "# >>> ccm >>>") != 1 {
		t.Fatalf("replace:\n%s", repl)
	}
	if out, _, _ := MergeTmux("", block); out != block {
		t.Fatalf("vacío:\n%q", out)
	}
}

func TestValidKey(t *testing.T) {
	for k, ok := range map[string]bool{"C-g": true, "M-c": true, "F5": true, "": false, "C-g; kill-server": false, "a b": false} {
		if ValidKey(k) != ok {
			t.Fatalf("%q", k)
		}
	}
}

func TestStrayLines(t *testing.T) {
	conf := "set -g mouse on\nbind-key -n C-g display-popup -E \"$HOME/go/bin/ccm\"\n" +
		TmuxBlock(bin, "C-g") + "set-hook -g client-session-changed 'run-shell -b \"$HOME/go/bin/ccm seen\"'\n"
	if got := StrayLines(conf); len(got) != 2 || got[0] != 2 || got[1] != 8 {
		t.Fatalf("got %v", got)
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	o := Options{Bin: bin, Settings: dir + "/settings.json", TmuxConf: dir + "/tmux.conf", Key: "C-g", DryRun: true}
	var out strings.Builder
	if err := Run(o, &out); err != nil {
		t.Fatal(err)
	}
	if exists(o.Settings) || exists(o.TmuxConf) {
		t.Fatal("dry-run escribió")
	}
	if !strings.Contains(out.String(), "C-g") {
		t.Fatalf("salida: %s", out.String())
	}
}

func TestRunWritesBackupAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	o := Options{Bin: bin, Settings: dir + "/settings.json", TmuxConf: dir + "/tmux.conf", Key: "C-g"}
	writeFile(t, o.Settings, `{"model":"x"}`)
	var out strings.Builder
	if err := Run(o, &out); err != nil {
		t.Fatal(err)
	}
	if b := readFile(t, o.Settings+".bak-ccm"); b != `{"model":"x"}` {
		t.Fatalf("backup = %q", b)
	}
	if len(commands(t, []byte(readFile(t, o.Settings)), "Stop")) != 1 || !strings.Contains(readFile(t, o.TmuxConf), "bind-key -n C-g") {
		t.Fatal("no escribió")
	}
	first := readFile(t, o.Settings) + readFile(t, o.TmuxConf)
	out.Reset()
	if err := Run(o, &out); err != nil {
		t.Fatal(err)
	}
	if readFile(t, o.Settings)+readFile(t, o.TmuxConf) != first || !strings.Contains(out.String(), "already") {
		t.Fatalf("segunda corrida cambió algo:\n%s", out.String())
	}
}

func TestRunRefusesGoRunBinary(t *testing.T) {
	o := Options{Bin: "/var/folders/x/go-build123/b001/exe/ccm", Settings: t.TempDir() + "/s.json", TmuxConf: t.TempDir() + "/t", Key: "C-g"}
	if err := Run(o, &strings.Builder{}); err == nil {
		t.Fatal("aceptó un binario temporal de go run")
	}
}

func TestRunRejectsBadKey(t *testing.T) {
	o := Options{Bin: bin, Settings: t.TempDir() + "/s.json", TmuxConf: t.TempDir() + "/t", Key: "C-g; kill-server"}
	if err := Run(o, &strings.Builder{}); err == nil {
		t.Fatal("aceptó una tecla inválida")
	}
}

func writeFile(t *testing.T, p, s string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRunKeepsFileModes(t *testing.T) {
	dir := t.TempDir()
	o := Options{Bin: bin, Settings: dir + "/settings.json", TmuxConf: dir + "/tmux.conf", Key: "C-g"}
	if err := os.WriteFile(o.Settings, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Run(o, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{o.Settings, o.Settings + ".bak-ccm"} {
		if fi, _ := os.Stat(p); fi.Mode().Perm() != 0o600 {
			t.Fatalf("%s: %v", p, fi.Mode().Perm())
		}
	}
}

func TestMergeTmuxBrokenMarkersFails(t *testing.T) {
	block := TmuxBlock(bin, "C-g")
	for _, conf := range []string{
		"A\n# >>> ccm >>>\nUSER_LINE\n",
		"# <<< ccm <<<\nA\n# >>> ccm >>>\n",
		block + block,
	} {
		if _, _, err := MergeTmux(conf, block); err == nil {
			t.Fatalf("aceptó marcadores rotos:\n%s", conf)
		}
	}
}

func TestTmuxBlockQuotesBinWithSpaces(t *testing.T) {
	b := TmuxBlock("/Users/John Smith/go/bin/ccm", "C-g")
	for _, want := range []string{
		`display-popup -E -h "60%" -w "80%" "'/Users/John Smith/go/bin/ccm'"`,
		`set-hook -g session-window-changed[42] "run-shell -b \"'/Users/John Smith/go/bin/ccm' seen\""`,
	} {
		if !strings.Contains(b, want) {
			t.Fatalf("falta %q en:\n%s", want, b)
		}
	}
}

func TestRunRejectsUnsafeBinPath(t *testing.T) {
	for _, b := range []string{`/a"b/ccm`, `/a'b/ccm`, `/a$b/ccm`, "/a#b/ccm", `/a\b/ccm`} {
		o := Options{Bin: b, Settings: t.TempDir() + "/s.json", TmuxConf: t.TempDir() + "/t", Key: "C-g"}
		if err := Run(o, &strings.Builder{}); err == nil {
			t.Fatalf("aceptó %q", b)
		}
	}
}

func TestMergeHooksDuplicateKeysFails(t *testing.T) {
	if _, _, err := MergeHooks([]byte(`{"hooks":{},"hooks":{"Stop":[]}}`), bin); err == nil {
		t.Fatal("aceptó claves duplicadas")
	}
}

func TestMergeHooksKeepsUserEmptyGroup(t *testing.T) {
	out, _, _ := MergeHooks([]byte(`{"hooks":{"Stop":[{"matcher":"x","hooks":[]}]}}`), bin)
	if g := hooksOf(t, out)["Stop"]; len(g) != 2 || g[0].Matcher != "x" {
		t.Fatalf("Stop = %+v", g)
	}
}

func TestRunKeepsFirstBackup(t *testing.T) {
	dir := t.TempDir()
	o := Options{Bin: bin, Settings: dir + "/settings.json", TmuxConf: dir + "/tmux.conf", Key: "C-g"}
	writeFile(t, o.Settings, `{"model":"original"}`)
	writeFile(t, o.TmuxConf, "set -g mouse on\n")
	_ = Run(o, &strings.Builder{})
	o.Bin = "/new/ccm"
	_ = Run(o, &strings.Builder{})
	if b := readFile(t, o.Settings+".bak-ccm"); b != `{"model":"original"}` {
		t.Fatalf("backup pisado: %q", b)
	}
	if b := readFile(t, o.TmuxConf+".bak-ccm"); b != "set -g mouse on\n" {
		t.Fatalf("backup tmux: %q", b)
	}
}
