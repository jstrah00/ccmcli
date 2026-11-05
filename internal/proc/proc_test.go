package proc

import (
	"errors"
	"os"
	"testing"
)

var hostArgs = []string{
	"/x/claude", "--bg-pty-host", "/tmp/d/pty/2e1e.sock", "212", "61", "--",
	"/x/2.1.318", "--session-id", "2e1e", "--fork-session",
	"--resume", "/h/.claude/projects/-p/a05a.jsonl", "--model", "m",
}

func TestParseBgHost(t *testing.T) {
	h, ok := ParseBgHost(hostArgs)
	if !ok || h.SessionID != "2e1e" || h.Resumed != "a05a" {
		t.Fatalf("got %+v, %v", h, ok)
	}
	if _, ok := ParseBgHost([]string{"claude", "--resume"}); ok {
		t.Fatal("un claude común no es pty-host")
	}
	if _, ok := ParseBgHost([]string{"claude", "--bg-pty-host", "s"}); ok {
		t.Fatal("pty-host sin --session-id no sirve")
	}
}

func TestFindBgHostWalksAncestors(t *testing.T) {
	procs := map[int]struct {
		ppid int
		args []string
	}{
		100: {50, []string{"ccm", "hook", "ask"}},
		50:  {40, []string{"/bin/sh", "-c", "ccm hook ask"}},
		40:  {30, hostArgs},
	}
	look := func(pid int) (int, []string, error) {
		p, ok := procs[pid]
		if !ok {
			return 0, nil, errors.New("no existe")
		}
		return p.ppid, p.args, nil
	}
	h, ok := FindBgHost(100, look)
	if !ok || h.SessionID != "2e1e" {
		t.Fatalf("got %+v, %v", h, ok)
	}
	delete(procs, 40)
	if _, ok := FindBgHost(100, look); ok {
		t.Fatal("encontró host inexistente")
	}
}

func TestPSSelf(t *testing.T) {
	ppid, args, err := PS(os.Getpid())
	if err != nil || ppid != os.Getppid() || len(args) == 0 {
		t.Fatalf("ppid=%d args=%v err=%v", ppid, args, err)
	}
}
