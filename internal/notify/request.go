// Package notify manda los avisos de escritorio de ccm.
package notify

type Kind string

const (
	Input    Kind = "input"
	Finished Kind = "finished"
)

type Request struct {
	Kind      Kind   `json:"kind"`
	PaneID    string `json:"pane_id"`
	SessionID string `json:"session_id"`
	Session   string `json:"session"`
	Window    int    `json:"window"`
	Folder    string `json:"folder"`
	Body      string `json:"body"`
}
