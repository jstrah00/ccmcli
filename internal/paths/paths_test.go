package paths

import (
	"path/filepath"
	"testing"
)

func TestDirsRespectXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/c")
	t.Setenv("XDG_STATE_HOME", "/s")
	checks := map[string]string{
		PanesDir():  "/c/ccm/panes",
		TitlesDir(): "/c/ccm/titles",
		LogPath():   "/c/ccm/log",
		OrderPath(): "/s/ccm/order.json",
	}
	for got, want := range checks {
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestDirsDefaultToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("XDG_STATE_HOME", "")
	if got, want := CacheDir(), filepath.Join(home, ".cache", "ccm"); got != want {
		t.Errorf("CacheDir = %q, want %q", got, want)
	}
	if got, want := StateDir(), filepath.Join(home, ".local", "state", "ccm"); got != want {
		t.Errorf("StateDir = %q, want %q", got, want)
	}
}
