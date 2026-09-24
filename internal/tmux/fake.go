package tmux

// Fake implementa Client en memoria para tests.
type Fake struct {
	Panes    []PaneInfo
	Active   map[string]bool
	Switched []PaneInfo
	Err      error
}

func (f *Fake) ListPanes() ([]PaneInfo, error)         { return f.Panes, f.Err }
func (f *Fake) PaneActive(paneID string) (bool, error) { return f.Active[paneID], f.Err }
func (f *Fake) Switch(p PaneInfo) error {
	f.Switched = append(f.Switched, p)
	return f.Err
}
