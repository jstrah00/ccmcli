// Package config lee la configuración opcional de ccm.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

type Notify struct {
	Banner         bool   `json:"banner"`
	Sound          bool   `json:"sound"`
	MinTurnSeconds int    `json:"min_turn_seconds"`
	TerminalBundle string `json:"terminal_bundle"`
}

type Config struct {
	Notify Notify `json:"notify"`
}

func Default() Config {
	return Config{Notify: Notify{Banner: true, Sound: true, MinTurnSeconds: 10}}
}

// Load devuelve los defaults si el archivo no existe o está roto; en el
// segundo caso también el error, para loguearlo.
func Load(path string) (Config, error) {
	c := Default()
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return Default(), fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
