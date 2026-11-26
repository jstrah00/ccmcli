package hook

import (
	"strings"
	"testing"
	"time"

	"github.com/jstrah00/ccmcli/internal/notify"
	"github.com/jstrah00/ccmcli/internal/tmux"
)

func notifyDeps(t *testing.T, active bool) (Deps, *[]notify.Request) {
	d := deps(t, active)
	var got []notify.Request
	d.Notify = func(r notify.Request) { got = append(got, r) }
	d.MinTurn = 10 * time.Second
	d.PaneLoc = func(string) (tmux.PaneInfo, error) {
		return tmux.PaneInfo{Session: "work", Window: 2}, nil
	}
	return d, &got
}

func at(d *Deps, off time.Duration) { d.Now = func() time.Time { return t0.Add(off) } }

func TestNotifyPermissionWhenNotLooking(t *testing.T) {
	d, got := notifyDeps(t, false)
	in := `{"session_id":"s1","cwd":"/home/u/proj","message":"Claude needs your permission to use Bash"}`
	if err := Run("permission", strings.NewReader(in), d); err != nil {
		t.Fatal(err)
	}
	want := notify.Request{Kind: notify.Input, PaneID: "%5", SessionID: "s1", Session: "work", Window: 2,
		Folder: "proj", Body: "Claude needs your permission to use Bash"}
	if len(*got) != 1 || (*got)[0] != want {
		t.Fatalf("got %+v", *got)
	}
}

func TestNoNotifyWhenLooking(t *testing.T) {
	for _, ev := range []string{"permission", "ask", "stop"} {
		d, got := notifyDeps(t, true)
		if err := Run(ev, strings.NewReader(input), d); err != nil {
			t.Fatal(err)
		}
		if len(*got) != 0 {
			t.Fatalf("%s: avisó mirando el pane: %+v", ev, *got)
		}
	}
}

func TestNotifyAskUsesFirstQuestion(t *testing.T) {
	d, got := notifyDeps(t, false)
	in := `{"session_id":"s1","cwd":"/p","tool_input":{"questions":[{"question":"Which DB?"},{"question":"x"}]}}`
	_ = Run("ask", strings.NewReader(in), d)
	if len(*got) != 1 || (*got)[0].Kind != notify.Input || (*got)[0].Body != "Which DB?" {
		t.Fatalf("got %+v", *got)
	}
}

func TestStopNotifiesAfterMinTurn(t *testing.T) {
	d, got := notifyDeps(t, false)
	_ = Run("prompt", strings.NewReader(input), d)
	at(&d, 11*time.Second)
	in := `{"session_id":"s1","cwd":"/home/u/proj","last_assistant_message":"Done.\n\nAll   tests pass."}`
	_ = Run("stop", strings.NewReader(in), d)
	if len(*got) != 1 || (*got)[0].Kind != notify.Finished || (*got)[0].Body != "Done. All tests pass." {
		t.Fatalf("got %+v", *got)
	}
}

func TestStopSkipsShortTurn(t *testing.T) {
	d, got := notifyDeps(t, false)
	_ = Run("prompt", strings.NewReader(input), d)
	at(&d, 3*time.Second)
	_ = Run("stop", strings.NewReader(input), d)
	if len(*got) != 0 {
		t.Fatalf("avisó un turno corto: %+v", *got)
	}
}

func TestStopWithoutTurnStartNotifies(t *testing.T) {
	d, got := notifyDeps(t, false)
	_ = Run("stop", strings.NewReader(input), d)
	if len(*got) != 1 {
		t.Fatalf("got %+v", *got)
	}
}

func TestInjectedPromptKeepsTurnStart(t *testing.T) {
	d, _ := notifyDeps(t, false)
	_ = Run("prompt", strings.NewReader(input), d)
	at(&d, 5*time.Second)
	_ = Run("prompt", strings.NewReader(`{"session_id":"s1","prompt":"<task-notification>x"}`), d)
	if p := load(t, d); !p.TurnStart.Equal(t0) {
		t.Fatalf("TurnStart = %v", p.TurnStart)
	}
}

func TestBodyTruncated(t *testing.T) {
	d, got := notifyDeps(t, false)
	in := `{"session_id":"s1","last_assistant_message":"` + strings.Repeat("a", 200) + `"}`
	_ = Run("stop", strings.NewReader(in), d)
	if len(*got) != 1 || len([]rune((*got)[0].Body)) != 110 {
		t.Fatalf("got %+v", *got)
	}
}

