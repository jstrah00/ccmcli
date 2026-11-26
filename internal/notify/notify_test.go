package notify

import (
	"slices"
	"strings"
	"testing"
)

var req = Request{Kind: Finished, PaneID: "%5", SessionID: "s1", Session: "work", Window: 2, Folder: "api", Body: "All tests pass."}

func TestTitle(t *testing.T) {
	if got := Title(req); got != "work:2/api" {
		t.Fatalf("got %q", got)
	}
	r := req
	r.Folder = "work"
	if got := Title(r); got != "work:2" {
		t.Fatalf("got %q", got)
	}
	r = Request{Folder: "api"}
	if got := Title(r); got != "api" {
		t.Fatalf("sin tmux: got %q", got)
	}
}

func TestSubtitleAndSound(t *testing.T) {
	if Subtitle(Input) != "Needs your input" || Subtitle(Finished) != "Finished" {
		t.Fatal("subtitles")
	}
	if Sound(Input) != "Sosumi" || Sound(Finished) != "Glass" {
		t.Fatal("sounds")
	}
}

func TestBundlePrecedence(t *testing.T) {
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }
	if b := Bundle("com.cfg", env(map[string]string{"__CFBundleIdentifier": "com.env"})); b != "com.cfg" {
		t.Fatalf("cfg: %q", b)
	}
	if b := Bundle("", env(map[string]string{"__CFBundleIdentifier": "com.env", "TERM_PROGRAM": "ghostty"})); b != "com.env" {
		t.Fatalf("env: %q", b)
	}
	for prog, want := range map[string]string{
		"ghostty": "com.mitchellh.ghostty", "iTerm.app": "com.googlecode.iterm2", "Apple_Terminal": "com.apple.Terminal",
		"WezTerm": "com.github.wez.wezterm", "kitty": "net.kovidgoyal.kitty", "WarpTerminal": "dev.warp.Warp-Stable",
		"vscode": "com.microsoft.VSCode", "tmux": "",
	} {
		if b := Bundle("", env(map[string]string{"TERM_PROGRAM": prog})); b != want {
			t.Fatalf("%s: %q", prog, b)
		}
	}
}

func arg(args []string, flag string) (string, bool) {
	i := slices.Index(args, flag)
	if i < 0 || i+1 >= len(args) {
		return "", false
	}
	return args[i+1], true
}

func TestMacArgs(t *testing.T) {
	a := MacArgs(req, "com.mitchellh.ghostty", "/opt/homebrew/bin/tmux")
	checks := map[string]string{
		"-title": "work:2/api", "-subtitle": "Finished", "-message": "All tests pass.",
		"-group": "claude-s1", "-activate": "com.mitchellh.ghostty",
		"-execute": "'/opt/homebrew/bin/tmux' switch-client -t '%5'",
	}
	for f, want := range checks {
		if got, _ := arg(a, f); got != want {
			t.Fatalf("%s = %q, want %q", f, got, want)
		}
	}
	if slices.Contains(a, "-sender") {
		t.Fatal("-sender hace que el click ignore -execute")
	}
}

func TestMacArgsWithoutBundleOrPane(t *testing.T) {
	r := req
	r.PaneID = "%5'; rm -rf ~"
	a := MacArgs(r, "", "/tmux")
	if _, ok := arg(a, "-activate"); ok {
		t.Fatal("-activate sin bundle")
	}
	if _, ok := arg(a, "-execute"); ok {
		t.Fatal("-execute con pane inválido")
	}
}

func TestMacArgsEscapesLeadingSymbol(t *testing.T) {
	for _, body := range []string{"[x] done", "(a)", "-n", "\"q\""} {
		r := req
		r.Body = body
		if got, _ := arg(MacArgs(r, "", ""), "-message"); got != `\`+body {
			t.Fatalf("%q -> %q", body, got)
		}
	}
}

func TestEmptyBodyUsesSubtitle(t *testing.T) {
	r := req
	r.Body = ""
	if got, _ := arg(MacArgs(r, "", ""), "-message"); got != "Finished" {
		t.Fatalf("got %q", got)
	}
}

func TestOsaArgs(t *testing.T) {
	a := OsaArgs(req)
	if !slices.Equal(a[len(a)-3:], []string{"work:2/api", "Finished", "All tests pass."}) {
		t.Fatalf("argv: %q", a)
	}
	if strings.Contains(strings.Join(a[:len(a)-3], " "), "All tests") {
		t.Fatal("el texto no va interpolado en el AppleScript")
	}
}

func TestLinuxArgs(t *testing.T) {
	a := LinuxArgs(req)
	for _, want := range []string{"--app-name=ccm", "--action=go=Go to session", "--wait", "work:2/api · Finished", "All tests pass."} {
		if !slices.Contains(a, want) {
			t.Fatalf("falta %q en %q", want, a)
		}
	}
}

func TestMacArgsSocketAndQuoting(t *testing.T) {
	r := req
	r.Socket = "/tmp/tmux-501/my sock"
	got, _ := arg(MacArgs(r, "", "/opt/it's/tmux"), "-execute")
	want := `'/opt/it'\''s/tmux' -S '/tmp/tmux-501/my sock' switch-client -t '%5'`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestSocketFromTMUX(t *testing.T) {
	if s := SocketFromTMUX("/private/tmp/tmux-501/default,1234,0"); s != "/private/tmp/tmux-501/default" {
		t.Fatalf("got %q", s)
	}
	if s := SocketFromTMUX(""); s != "" {
		t.Fatalf("got %q", s)
	}
}

func TestSwitchArgs(t *testing.T) {
	r := req
	r.Socket = "/s"
	if a := SwitchArgs(r); !slices.Equal(a, []string{"-S", "/s", "switch-client", "-t", "%5"}) {
		t.Fatalf("got %q", a)
	}
}

func TestLinuxArgsPlain(t *testing.T) {
	a := LinuxArgsPlain(req)
	if slices.Contains(a, "--wait") || slices.ContainsFunc(a, func(s string) bool { return strings.HasPrefix(s, "--action") }) {
		t.Fatalf("got %q", a)
	}
}
