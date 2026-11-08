package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestActive(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"1.json": `{"pid":1,"sessionId":"on","bridgeSessionId":"session_x"}`,
		"2.json": `{"pid":2,"sessionId":"off","bridgeSessionId":null}`,
		"3.json": `{"pid":3,"sessionId":"never"}`,
		"4.json": `{roto`,
		"5.key":  `{"sessionId":"key","bridgeSessionId":"session_y"}`,
	}
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := Active(dir)
	if len(got) != 1 || !got["on"] {
		t.Fatalf("Active = %v, quiero solo on", got)
	}
}

func TestActiveSinDir(t *testing.T) {
	if got := Active(filepath.Join(t.TempDir(), "no")); len(got) != 0 {
		t.Fatalf("Active = %v", got)
	}
}
