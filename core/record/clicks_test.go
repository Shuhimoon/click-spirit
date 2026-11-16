package record

import (
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

func TestClicksFromEventsOrderTimecodesAndBubble(t *testing.T) {
	t0 := time.Date(2026, 11, 16, 21, 0, 0, 0, time.UTC)
	at := func(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }
	bubble := core.Rect{X: 1500, Y: 900, W: 60, H: 60}
	// Deliberately out of order.
	events := []Event{
		{Kind: KindUp, Button: core.ButtonLeft, X: 300, Y: 400, Time: at(2550)},
		{Kind: KindDown, Button: core.ButtonLeft, X: 100, Y: 200, Time: at(1000)},
		{Kind: KindUp, Button: core.ButtonLeft, X: 100, Y: 200, Time: at(1080)},
		{Kind: KindDown, Button: core.ButtonLeft, X: 1510, Y: 910, Time: at(1500)}, // bubble
		{Kind: KindUp, Button: core.ButtonLeft, X: 1510, Y: 910, Time: at(1560)},
		{Kind: KindMove, X: 5, Y: 5, Time: at(1600)},
		{Kind: KindDown, Button: core.ButtonLeft, X: 300, Y: 400, Time: at(2500)},
		{Kind: KindDown, Button: core.ButtonRight, X: 50, Y: 60, Time: at(4000)},
		{Kind: KindUp, Button: core.ButtonRight, X: 50, Y: 60, Time: at(4010)},
	}
	got := ClicksFromEvents(events, at(500), []core.Rect{bubble})
	want := []TimedClick{
		{X: 100, Y: 200, Button: core.ButtonLeft, AtMs: 500, HoldMs: 80},
		{X: 300, Y: 400, Button: core.ButtonLeft, AtMs: 2000, HoldMs: 50},
		{X: 50, Y: 60, Button: core.ButtonRight, AtMs: 3500, HoldMs: 10},
	}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d] = %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestScriptFromClicksIntervalsOrderAndExclude(t *testing.T) {
	bubble := core.Rect{X: 0, Y: 0, W: 50, H: 50}
	clicks := []TimedClick{
		{X: 300, Y: 400, AtMs: 2000, HoldMs: 50},
		{X: 100, Y: 200, AtMs: 500, HoldMs: 80},
		{X: 10, Y: 10, AtMs: 1200}, // on the bubble
		{X: 50, Y: 60, Button: core.ButtonRight, AtMs: 3500, HoldMs: 10},
		{X: 51, Y: 61, AtMs: 3505},
	}
	s, err := ScriptFromClicks(clicks, "", []core.Rect{bubble}, 300)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	type st struct{ x, y, iv, hold int }
	want := []st{
		{100, 200, 2000 - 500 - 80, 80},
		{300, 400, 3500 - 2000 - 50, 50},
		{50, 60, 1, 10}, // 3505-3500-10 < 1 -> 1 (back to back)
		{51, 61, 300, 0},
	}
	if len(s.Steps) != len(want) {
		t.Fatalf("steps %+v", s.Steps)
	}
	for i, w := range want {
		g := s.Steps[i]
		if g.X != w.x || g.Y != w.y || g.IntervalMs != w.iv || g.HoldMs != w.hold || g.Clicks != 1 || g.Target != nil {
			t.Fatalf("step %d = %+v want %+v", i, g, w)
		}
	}
	if s.Steps[2].Button != core.ButtonRight || s.Steps[0].Button != core.ButtonLeft {
		t.Fatal("buttons not kept")
	}
	if s.Name != "錄影腳本" || s.EffectiveStartDelaySec() != 3 {
		t.Fatalf("name %q delay %d", s.Name, s.EffectiveStartDelaySec())
	}
	if _, err := ScriptFromClicks([]TimedClick{{X: 1, Y: 1}}, "x", []core.Rect{{X: 0, Y: 0, W: 5, H: 5}}, 0); err == nil {
		t.Fatal("all-excluded clicks should fail")
	}
}

func TestFilterExcludedDropsBubblePairs(t *testing.T) {
	bubble := core.Rect{X: 100, Y: 100, W: 40, H: 40}
	f := &FilterExcluded{Exclude: func(x, y int) bool { return bubble.Contains(x, y) }}
	evs := []Event{
		{Kind: KindDown, Button: core.ButtonLeft, X: 110, Y: 110},
		{Kind: KindUp, Button: core.ButtonLeft, X: 300, Y: 300}, // drag off the bubble
		{Kind: KindDown, Button: core.ButtonLeft, X: 10, Y: 10},
		{Kind: KindUp, Button: core.ButtonLeft, X: 10, Y: 10},
		{Kind: KindClick, Button: core.ButtonLeft, X: 120, Y: 120},
	}
	var kept []Event
	for _, ev := range evs {
		if f.Keep(ev) {
			kept = append(kept, ev)
		}
	}
	if len(kept) != 2 || kept[0].X != 10 || kept[1].Kind != KindUp {
		t.Fatalf("kept %+v", kept)
	}
	s, err := ToScript(append(evs[:0:0], evs...), Options{Exclude: []core.Rect{bubble}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Steps) != 1 || s.Steps[0].X != 10 {
		t.Fatalf("ToScript with Exclude: %+v", s.Steps)
	}
}
