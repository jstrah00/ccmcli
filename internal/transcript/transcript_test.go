package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func aiLine(s string) string {
	return fmt.Sprintf(`{"type":"ai-title","aiTitle":%q,"sessionId":"s1"}`, s)
}

func feedAll(lines ...string) Titles {
	var t Titles
	for _, l := range lines {
		t.Feed([]byte(l))
	}
	return t
}

func TestFeedFirstAndLastAITitle(t *testing.T) {
	got := feedAll(
		`{"type":"user","message":"hablo de \"ai-title\" en el texto"}`,
		aiLine("A"),
		aiLine("B"),
		`{"type":"ai-title"`, // línea rota
	)
	if got.FirstAITitle != "A" || got.LastAITitle != "B" || got.Row() != "A" {
		t.Fatalf("got %+v", got)
	}
}

func TestAgentNameWinsRow(t *testing.T) {
	got := feedAll(aiLine("A"), `{"type":"agent-name","agentName":"X"}`, aiLine("B"))
	if got.Row() != "X" || got.LastAITitle != "B" {
		t.Fatalf("got %+v", got)
	}
}

func TestEmptyTitles(t *testing.T) {
	if got := feedAll(`{"type":"user"}`); got.Row() != "" {
		t.Fatalf("Row = %q", got.Row())
	}
}

func TestCacheIncrementalAndPartialLine(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "s1.jsonl")
	// B todavía sin terminar de escribir
	os.WriteFile(tr, []byte(aiLine("A")+"\n"+`{"type":"ai-title","aiTitle":"B"`), 0o644)
	c := NewCache(filepath.Join(dir, "cache"))

	got := c.Get("s1", tr)
	if got.FirstAITitle != "A" || got.LastAITitle != "A" {
		t.Fatalf("con línea parcial: %+v", got)
	}

	f, _ := os.OpenFile(tr, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString(`,"sessionId":"s1"}` + "\n")
	f.Close()
	got = c.Get("s1", tr)
	if got.FirstAITitle != "A" || got.LastAITitle != "B" {
		t.Fatalf("tras completar la línea: %+v", got)
	}
}

func TestCachePersistsOnDisk(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "s1.jsonl")
	content := aiLine("A") + "\n" + aiLine("B") + "\n"
	os.WriteFile(tr, []byte(content), 0o644)
	NewCache(filepath.Join(dir, "cache")).Get("s1", tr)

	// Mismo tamaño, contenido basura: un caché nuevo no debería reescanear.
	os.WriteFile(tr, []byte(strings.Repeat("x", len(content))), 0o644)
	got := NewCache(filepath.Join(dir, "cache")).Get("s1", tr)
	if got.FirstAITitle != "A" || got.LastAITitle != "B" {
		t.Fatalf("no usó el caché en disco: %+v", got)
	}
}

func TestCacheRescansWhenFileShrinks(t *testing.T) {
	dir := t.TempDir()
	tr := filepath.Join(dir, "s1.jsonl")
	os.WriteFile(tr, []byte(aiLine("A")+"\n"+aiLine("B")+"\n"), 0o644)
	c := NewCache(filepath.Join(dir, "cache"))
	c.Get("s1", tr)

	os.WriteFile(tr, []byte(aiLine("C")+"\n"), 0o644)
	got := c.Get("s1", tr)
	if got.FirstAITitle != "C" || got.LastAITitle != "C" {
		t.Fatalf("got %+v", got)
	}
}

func TestCacheMissingFileOrSession(t *testing.T) {
	c := NewCache(t.TempDir())
	if got := c.Get("s1", "/no/existe.jsonl"); got != (Titles{}) {
		t.Fatalf("got %+v", got)
	}
	if got := c.Get("", "/x"); got != (Titles{}) {
		t.Fatalf("got %+v", got)
	}
}
