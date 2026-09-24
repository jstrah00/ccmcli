package hook

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"ccm/internal/state"
	"ccm/internal/tmux"
)

var t0 = time.Date(2026, 9, 23, 22, 0, 0, 0, time.UTC)

const input = `{"session_id":"s1","transcript_path":"/t/s1.jsonl","cwd":"/home/u/proj","prompt":"hola"}`

func deps(t *testing.T, active bool) Deps {
	return Deps{
		Store:      state.Store{Dir: t.TempDir()},
		PaneID:     "%5",
		Now:        func() time.Time { return t0 },
		PaneActive: func(string) (bool, error) { return active, nil },
	}
}

func load(t *testing.T, d Deps) state.Pane {
	t.Helper()
	p, err := d.Store.Load(d.PaneID)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunTransitions(t *testing.T) {
	cases := []struct {
		event  string
		active bool
		want   state.State
	}{
		{"start", false, state.Idle},
		{"prompt", false, state.Working},
		{"ask", false, state.Waiting},
		{"permission", false, state.Waiting},
		{"tool-done", false, state.Working},
		{"stop", false, state.Done},
		{"stop", true, state.Idle},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/active=%v", c.event, c.active), func(t *testing.T) {
			d := deps(t, c.active)
			if err := Run(c.event, strings.NewReader(input), d); err != nil {
				t.Fatal(err)
			}
			p := load(t, d)
			if p.State != c.want {
				t.Fatalf("state = %q, want %q", p.State, c.want)
			}
			if p.SessionID != "s1" || p.Cwd != "/home/u/proj" || p.TranscriptPath != "/t/s1.jsonl" {
				t.Fatalf("campos no copiados: %+v", p)
			}
		})
	}
}

func TestRunEndDeletes(t *testing.T) {
	d := deps(t, false)
	_ = Run("prompt", strings.NewReader(input), d)
	if err := Run("end", strings.NewReader(input), d); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Store.Load("%5"); err == nil {
		t.Fatal("el estado sigue existiendo")
	}
}

func TestRunInjectedPromptKeepsState(t *testing.T) {
	d := deps(t, false)
	_ = Run("stop", strings.NewReader(input), d)
	for _, pr := range []string{"<task-notification>x", "<system-reminder>y"} {
		in := fmt.Sprintf(`{"session_id":"s1","prompt":%q}`, pr)
		if err := Run("prompt", strings.NewReader(in), d); err != nil {
			t.Fatal(err)
		}
		if p := load(t, d); p.State != state.Done {
			t.Fatalf("prompt inyectado %q cambió el estado a %q", pr, p.State)
		}
	}
}

func TestRunWithoutPaneIsNoop(t *testing.T) {
	d := deps(t, false)
	d.PaneID = ""
	if err := Run("prompt", strings.NewReader(input), d); err != nil {
		t.Fatal(err)
	}
	all, _ := d.Store.All()
	if len(all) != 0 {
		t.Fatalf("escribió estado sin pane: %+v", all)
	}
}

func TestRunUnknownEvent(t *testing.T) {
	err := Run("nope", strings.NewReader(input), deps(t, false))
	if !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("err = %v", err)
	}
}

func TestRunInvalidStdinStillTransitions(t *testing.T) {
	d := deps(t, false)
	if err := Run("prompt", strings.NewReader("{roto"), d); err != nil {
		t.Fatal(err)
	}
	if p := load(t, d); p.State != state.Working {
		t.Fatalf("state = %q", p.State)
	}
}

func TestRunKeepsFieldsWhenInputEmpty(t *testing.T) {
	d := deps(t, false)
	_ = Run("prompt", strings.NewReader(input), d)
	if err := Run("ask", strings.NewReader(""), d); err != nil {
		t.Fatal(err)
	}
	p := load(t, d)
	if p.SessionID != "s1" || p.State != state.Waiting {
		t.Fatalf("got %+v", p)
	}
}

// idle_prompt: CC quedó esperando un prompt nuevo. Corrige estados que
// quedaron pegados porque Stop no dispara al interrumpir con Esc.
func TestRunIdlePromptUnsticksState(t *testing.T) {
	cases := []struct {
		from   state.State
		active bool
		want   state.State
	}{
		{state.Working, false, state.Done},
		{state.Waiting, false, state.Done},
		{state.Working, true, state.Idle},
		{state.Done, false, state.Done},
		{state.Idle, false, state.Idle},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%s/active=%v", c.from, c.active), func(t *testing.T) {
			d := deps(t, c.active)
			_ = d.Store.Save(state.Pane{PaneID: "%5", SessionID: "s1", State: c.from, Since: t0.Add(-time.Hour)})
			if err := Run("idle", strings.NewReader(input), d); err != nil {
				t.Fatal(err)
			}
			p := load(t, d)
			if p.State != c.want {
				t.Fatalf("state = %q, want %q", p.State, c.want)
			}
			if c.from == c.want && !p.Since.Equal(t0.Add(-time.Hour)) {
				t.Fatalf("since cambió sin cambiar el estado")
			}
		})
	}
}

func TestRunIdlePromptWithoutStateIsNoop(t *testing.T) {
	d := deps(t, false)
	if err := Run("idle", strings.NewReader(input), d); err != nil {
		t.Fatal(err)
	}
	if all, _ := d.Store.All(); len(all) != 0 {
		t.Fatalf("creó estado: %+v", all)
	}
}

func TestSeenMarksDoneInActiveAttachedWindows(t *testing.T) {
	st := state.Store{Dir: t.TempDir()}
	_ = st.Save(state.Pane{PaneID: "%1", State: state.Done, Since: t0})
	_ = st.Save(state.Pane{PaneID: "%2", State: state.Done, Since: t0})
	_ = st.Save(state.Pane{PaneID: "%3", State: state.Working, Since: t0})
	_ = st.Save(state.Pane{PaneID: "%4", State: state.Done, Since: t0})
	fake := &tmux.Fake{Panes: []tmux.PaneInfo{
		{PaneID: "%1", WindowActive: true, SessionAttached: true},
		{PaneID: "%2", WindowActive: false, SessionAttached: true},
		{PaneID: "%3", WindowActive: true, SessionAttached: true},
		{PaneID: "%4", WindowActive: true, SessionAttached: false},
	}}
	if err := Seen(fake, st, t0.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	want := map[string]state.State{"%1": state.Idle, "%2": state.Done, "%3": state.Working, "%4": state.Done}
	for id, w := range want {
		p, _ := st.Load(id)
		if p.State != w {
			t.Errorf("%s = %q, want %q", id, p.State, w)
		}
	}
}
