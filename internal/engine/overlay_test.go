package engine

import (
	"encoding/json"
	"math"
	"strconv"
	"sync"
	"testing"

	"github.com/click-spirit/click-spirit/core"
)

type shownClick struct {
	x, y  int
	label string
}

type fakeOverlay struct {
	mu    sync.Mutex
	shown []shownClick
}

func (f *fakeOverlay) ShowClick(x, y int, label string) {
	f.mu.Lock()
	f.shown = append(f.shown, shownClick{x, y, label})
	f.mu.Unlock()
}

func (f *fakeOverlay) snapshot() []shownClick {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]shownClick(nil), f.shown...)
}

func clickPayloads(t *testing.T, e *Engine) []eventPayload {
	t.Helper()
	var out []eventPayload
	for {
		raw, ok := e.PollEvent()
		if !ok {
			return out
		}
		var p eventPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		if p.Kind == string(core.EventClick) {
			out = append(out, p)
		}
	}
}

func overlayScript(t *testing.T) string {
	return scriptJSON(t, &core.Script{
		Version: 1,
		Repeat:  1,
		Steps: []core.Step{
			{X: 30, Y: 40, Button: core.ButtonLeft},
			{Button: core.ButtonLeft, Repeat: 10, Target: &core.Target{Kind: core.TargetCircle, CX: 500, CY: 600, R: 25}},
		},
	})
}

func TestEngineQueuesClickEventsAndDrivesOverlay(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	e.Sleep = instant
	ov := &fakeOverlay{}
	e.SetOverlay(ov)
	if !e.ShowClicks() {
		t.Fatal("overlay should be on by default")
	}
	if err := e.Start(overlayScript(t)); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}

	clicks := clickPayloads(t, e)
	shown := ov.snapshot()
	if len(clicks) != 11 || len(shown) != 11 {
		t.Fatalf("got %d click events and %d overlay draws, want 11 each", len(clicks), len(shown))
	}
	if c := clicks[0]; c.Step != 0 || c.Repetition != 0 || c.Repeats != 1 || c.X != 30 || c.Y != 40 {
		t.Fatalf("first click payload %+v", c)
	}
	if shown[0] != (shownClick{30, 40, "步驟 1"}) {
		t.Fatalf("first overlay draw %+v", shown[0])
	}
	for k := 1; k < 11; k++ {
		c := clicks[k]
		if c.Step != 1 || c.Repetition != k-1 || c.Repeats != 10 {
			t.Fatalf("click %d payload %+v", k, c)
		}
		if d := math.Hypot(float64(c.X-500), float64(c.Y-600)); d > 25.75 {
			t.Fatalf("circle click %d at (%d,%d) is outside r=25", k, c.X, c.Y)
		}
		want := shownClick{c.X, c.Y, ""}
		want.label = "步驟 2 · 第 " + strconv.Itoa(k) + "/10 次"
		if shown[k] != want {
			t.Fatalf("overlay draw %d = %+v, want %+v", k, shown[k], want)
		}
	}
}

func TestEngineShowClicksOffSkipsOverlayButKeepsEvents(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	e.Sleep = instant
	ov := &fakeOverlay{}
	e.SetOverlay(ov)
	e.SetShowClicks(false)
	if err := e.Start(overlayScript(t)); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	if n := len(ov.snapshot()); n != 0 {
		t.Fatalf("overlay drew %d times while disabled", n)
	}
	if n := len(clickPayloads(t, e)); n != 11 {
		t.Fatalf("got %d click events, want 11", n)
	}
}

func TestClickLabel(t *testing.T) {
	cases := []struct {
		ev   core.Event
		want string
	}{
		{core.Event{Kind: core.EventClick, StepIndex: 1, Repetition: 2, Repeats: 10}, "步驟 2 · 第 3/10 次"},
		{core.Event{Kind: core.EventClick, StepIndex: 0, Repeats: 1}, "步驟 1"},
	}
	for _, c := range cases {
		if got := ClickLabel(c.ev); got != c.want {
			t.Fatalf("ClickLabel = %q, want %q", got, c.want)
		}
	}
}
