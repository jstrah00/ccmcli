package config

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, s string) string {
	p := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

var defaults = Notify{Banner: true, Sound: true, MinTurnSeconds: 10}

func TestLoadMissingIsDefaults(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil || c.Notify != defaults {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestLoadPartialKeepsDefaults(t *testing.T) {
	c, err := Load(write(t, `{"notify":{"sound":false,"terminal_bundle":"com.x"}}`))
	want := Notify{Banner: true, Sound: false, MinTurnSeconds: 10, TerminalBundle: "com.x"}
	if err != nil || c.Notify != want {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestLoadBrokenIsDefaultsWithError(t *testing.T) {
	c, err := Load(write(t, `{nope`))
	if err == nil || c.Notify != defaults {
		t.Fatalf("got %+v, %v", c, err)
	}
}
