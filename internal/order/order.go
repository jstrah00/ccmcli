// Package order persiste el orden manual de las entradas y cuáles están ocultas.
package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"ccm/internal/fsutil"
)

// Key identifica una entrada entre reinicios de tmux y /clear de CC.
// Si se renombra la sesión o se renumeran windows, la entrada va al final.
func Key(session string, window, pane int, cwd string) string {
	return fmt.Sprintf("%s:%d.%d|%s", session, window, pane, cwd)
}

type Store struct {
	Order    []string          `json:"order"`
	Hidden   []string          `json:"hidden"`
	Aliases  map[string]string `json:"aliases,omitempty"`
	Statuses map[string]string `json:"statuses,omitempty"`
}

func Load(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Store{}, nil
	}
	if err != nil {
		return &Store{}, err
	}
	var s Store
	if err := json.Unmarshal(b, &s); err != nil {
		return &Store{}, fmt.Errorf("order.json corrupto: %w", err)
	}
	return &s, nil
}

func (s *Store) Save(path string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(path, b)
}

func (s *Store) hidden(k string) bool { return slices.Contains(s.Hidden, k) }

// Visible: primero las claves de Order que están vivas, después las vivas que
// Order no conoce (entradas nuevas), en el orden de live. Sin ocultas.
func (s *Store) Visible(live []string) []string {
	alive := make(map[string]bool, len(live))
	for _, k := range live {
		alive[k] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, k := range s.Order {
		if alive[k] && !seen[k] && !s.hidden(k) {
			out = append(out, k)
			seen[k] = true
		}
	}
	for _, k := range live {
		if !seen[k] && !s.hidden(k) {
			out = append(out, k)
			seen[k] = true
		}
	}
	return out
}

// materialize agrega a Order las entradas nuevas en la posición en la que ya
// se ven, para que pegar "debajo de" una de ellas funcione.
func (s *Store) materialize(live []string) {
	for _, k := range live {
		if !s.hidden(k) && !slices.Contains(s.Order, k) {
			s.Order = append(s.Order, k)
		}
	}
}

func remove(xs []string, k string) []string {
	var out []string
	for _, x := range xs {
		if x != k {
			out = append(out, x)
		}
	}
	return out
}

func (s *Store) Hide(live []string, key string) {
	s.materialize(live)
	s.Order = remove(s.Order, key)
	if !s.hidden(key) {
		s.Hidden = append(s.Hidden, key)
	}
}

// Paste inserta key debajo (below) o arriba de anchor y la saca de ocultas.
// Sin anchor (o anchor desconocido) va al final.
func (s *Store) Paste(live []string, key, anchor string, below bool) {
	if key == "" {
		return
	}
	s.Hidden = remove(s.Hidden, key)
	s.materialize(live)
	s.Order = remove(s.Order, key)
	i := slices.Index(s.Order, anchor)
	switch {
	case anchor == "" || i < 0:
		s.Order = append(s.Order, key)
	case below:
		s.Order = slices.Insert(s.Order, i+1, key)
	default:
		s.Order = slices.Insert(s.Order, i, key)
	}
}

func (s *Store) RestoreAll(live []string) {
	s.materialize(live)
	for _, h := range s.Hidden {
		if !slices.Contains(s.Order, h) {
			s.Order = append(s.Order, h)
		}
	}
	s.Hidden = nil
}

func (s *Store) Alias(key string) string { return s.Aliases[key] }

// SetAlias con nombre vacío borra el alias.
func (s *Store) SetAlias(key, name string) { setIn(&s.Aliases, key, name) }

func (s *Store) Status(key string) string { return s.Statuses[key] }

// SetStatus con texto vacío borra el status.
func (s *Store) SetStatus(key, status string) { setIn(&s.Statuses, key, status) }

func setIn(m *map[string]string, key, v string) {
	v = strings.TrimSpace(v)
	if v == "" {
		delete(*m, key)
		return
	}
	if *m == nil {
		*m = map[string]string{}
	}
	(*m)[key] = v
}
