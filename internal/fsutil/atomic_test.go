package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicCreatesDirsAndReplaces(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a", "b", "f.json")
	if err := WriteFileAtomic(p, []byte("uno")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(p, []byte("dos")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "dos" {
		t.Fatalf("contenido = %q, want %q", b, "dos")
	}
	ents, _ := os.ReadDir(filepath.Dir(p))
	if len(ents) != 1 {
		t.Fatalf("quedaron temporales: %v", ents)
	}
}
