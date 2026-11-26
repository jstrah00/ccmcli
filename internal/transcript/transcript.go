// Package transcript saca los títulos de una sesión de su transcript JSONL.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/jstrah00/ccmcli/internal/fsutil"
)

type Titles struct {
	AgentName    string `json:"agent_name"`
	FirstAITitle string `json:"first_ai_title"`
	LastAITitle  string `json:"last_ai_title"`
}

// Row es el título de la fila: el nombre de /rename si hay; si no, el primer
// título autogenerado (el que describe con qué arrancó la sesión).
func (t Titles) Row() string {
	if t.AgentName != "" {
		return t.AgentName
	}
	return t.FirstAITitle
}

var (
	markAI    = []byte(`"ai-title"`)
	markAgent = []byte(`"agent-name"`)
)

// Feed procesa una línea del transcript. El prefiltro por bytes evita
// decodificar JSON en el 99% de las líneas.
func (t *Titles) Feed(line []byte) {
	if !bytes.Contains(line, markAI) && !bytes.Contains(line, markAgent) {
		return
	}
	var e struct {
		Type      string `json:"type"`
		AITitle   string `json:"aiTitle"`
		AgentName string `json:"agentName"`
	}
	if json.Unmarshal(line, &e) != nil {
		return
	}
	switch e.Type {
	case "ai-title":
		if e.AITitle == "" {
			return
		}
		if t.FirstAITitle == "" {
			t.FirstAITitle = e.AITitle
		}
		t.LastAITitle = e.AITitle
	case "agent-name":
		if e.AgentName != "" {
			t.AgentName = e.AgentName
		}
	}
}

type entry struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Titles Titles `json:"titles"`
}

// Cache lee cada transcript de forma incremental: recuerda hasta qué byte
// leyó (en memoria y en disco) y solo procesa lo nuevo.
type Cache struct {
	Dir string
	mem map[string]entry
}

func NewCache(dir string) *Cache { return &Cache{Dir: dir, mem: map[string]entry{}} }

func (c *Cache) Get(sessionID, path string) Titles {
	if sessionID == "" || path == "" {
		return Titles{}
	}
	e, ok := c.mem[sessionID]
	if !ok {
		e = c.loadDisk(sessionID)
	}
	if e.Path != path {
		e = entry{Path: path}
	}
	fi, err := os.Stat(path)
	if err != nil {
		c.mem[sessionID] = e
		return e.Titles
	}
	if fi.Size() < e.Offset {
		e = entry{Path: path} // se reescribió: empezar de nuevo
	}
	if fi.Size() > e.Offset {
		if n, err := scan(path, e.Offset, &e.Titles); err == nil && n > 0 {
			e.Offset += n
			c.saveDisk(sessionID, e)
		}
	}
	c.mem[sessionID] = e
	return e.Titles
}

// scan alimenta las líneas completas desde off y devuelve los bytes
// consumidos. Una última línea sin \n (CC escribiendo) no se consume.
func scan(path string, off int64, t *Titles) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}
	r := bufio.NewReaderSize(f, 64*1024)
	var n int64
	for {
		line, err := r.ReadBytes('\n')
		if err == io.EOF {
			return n, nil
		}
		if err != nil {
			return n, err
		}
		n += int64(len(line))
		t.Feed(line)
	}
}

func (c *Cache) diskPath(sessionID string) string {
	return filepath.Join(c.Dir, filepath.Base(sessionID)+".json")
}

func (c *Cache) loadDisk(sessionID string) entry {
	var e entry
	b, err := os.ReadFile(c.diskPath(sessionID))
	if err != nil || json.Unmarshal(b, &e) != nil {
		return entry{}
	}
	return e
}

func (c *Cache) saveDisk(sessionID string, e entry) {
	if b, err := json.Marshal(e); err == nil {
		_ = fsutil.WriteFileAtomic(c.diskPath(sessionID), b)
	}
}
