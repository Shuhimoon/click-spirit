package record

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

func TestToScriptMergeAndPlayback(t *testing.T) {
	base := time.Unix(1_700_000_000, 0)
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }
	events := []Event{
		{Kind: KindMove, X: 5, Y: 5, Time: at(0)},
		{Kind: KindDown, Button: core.ButtonLeft, X: 10, Y: 20, Time: at(100)},
		{Kind: KindUp, Button: core.ButtonLeft, X: 10, Y: 20, Time: at(160)},
		{Kind: KindDown, Button: core.ButtonLeft, X: 12, Y: 21, Time: at(200)},
		{Kind: KindUp, Button: core.ButtonLeft, X: 12, Y: 21, Time: at(230)},
		{Kind: KindClick, Button: core.ButtonRight, X: 400, Y: 500, Time: at(1000)},
		{Kind: KindDown, Button: core.ButtonMiddle, X: 1, Y: 1, Time: at(1500)},
		{Kind: KindUp, Button: core.ButtonMiddle, X: 1, Y: 1, Time: at(1580)},
		{Kind: KindUp, Button: core.ButtonRight, X: 9, Y: 9, Time: at(50)}, // orphan up
	}
	script, err := ToScript(events, Options{Name: "測試"})
	if err != nil {
		t.Fatal(err)
	}
	if script.Name != "測試" || script.Repeat != 1 || len(script.Steps) != 3 {
		t.Fatalf("%+v", script)
	}
	want := []core.Step{
		{X: 10, Y: 20, Button: core.ButtonLeft, Clicks: 2, IntervalMs: 770, HoldMs: 60},
		{X: 400, Y: 500, Button: core.ButtonRight, Clicks: 1, IntervalMs: 500, HoldMs: 0},
		{X: 1, Y: 1, Button: core.ButtonMiddle, Clicks: 1, IntervalMs: 0, HoldMs: 80},
	}
	for i := range want {
		if script.Steps[i] != want[i] {
			t.Fatalf("step %d\n got %+v\nwant %+v", i, script.Steps[i], want[i])
		}
	}
	if err := script.Validate(); err != nil {
		t.Fatal(err)
	}

	rec := &clickLog{}
	runner := &core.Runner{Clicker: rec, Sleep: rec.sleep}
	if err := runner.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	got := rec.snapshot()
	wantLog := []string{
		"move 10 20", "click left 2 60", "sleep 770",
		"move 400 500", "click right 1 0", "sleep 500",
		"move 1 1", "click middle 1 80",
	}
	if len(got) != len(wantLog) {
		t.Fatalf("%v", got)
	}
	for i := range wantLog {
		if got[i] != wantLog[i] {
			t.Fatalf("[%d]=%q want %q\n%v", i, got[i], wantLog[i], got)
		}
	}
}

func TestToScriptSpeedMinAndNoMerge(t *testing.T) {
	base := time.Unix(0, 0)
	at := func(ms int) time.Time { return base.Add(time.Duration(ms) * time.Millisecond) }
	events := []Event{
		{Kind: KindDown, Button: core.ButtonLeft, X: 10, Y: 20, Time: at(100)},
		{Kind: KindUp, Button: core.ButtonLeft, X: 10, Y: 20, Time: at(160)},
		{Kind: KindDown, Button: core.ButtonLeft, X: 12, Y: 21, Time: at(200)},
		{Kind: KindUp, Button: core.ButtonLeft, X: 12, Y: 21, Time: at(230)},
		{Kind: KindClick, Button: core.ButtonRight, X: 400, Y: 500, Time: at(1000)},
	}
	fast, err := ToScript(events, Options{Speed: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(fast.Steps) != 2 || fast.Steps[0].HoldMs != 30 || fast.Steps[0].IntervalMs != 385 || fast.Steps[0].Clicks != 2 {
		t.Fatalf("speed %+v", fast.Steps)
	}
	if fast.Steps[1].IntervalMs != 0 || fast.Steps[1].HoldMs != 0 {
		t.Fatalf("last %+v", fast.Steps[1])
	}

	floored, err := ToScript(events, Options{MinIntervalMs: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if floored.Steps[0].IntervalMs != 1000 || floored.Steps[1].IntervalMs != 1000 || floored.DefaultIntervalMs != 1000 {
		t.Fatalf("%+v", floored.Steps)
	}
	if floored.Steps[0].HoldMs != 60 {
		t.Fatal(floored.Steps[0].HoldMs)
	}

	split, err := ToScript(events, Options{DoubleClickMs: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(split.Steps) != 3 {
		t.Fatalf("split %+v", split.Steps)
	}
	if split.Steps[0].Clicks != 1 || split.Steps[0].IntervalMs != 40 || split.Steps[0].HoldMs != 60 {
		t.Fatalf("first %+v", split.Steps[0])
	}
	if split.Steps[1].Clicks != 1 || split.Steps[1].IntervalMs != 770 || split.Steps[1].HoldMs != 30 {
		t.Fatalf("second %+v", split.Steps[1])
	}
}

func TestToScriptCapsClicksAtThree(t *testing.T) {
	base := time.Unix(0, 0)
	var events []Event
	for i := 0; i < 4; i++ {
		events = append(events, Event{
			Kind: KindClick, Button: core.ButtonLeft, X: 1, Y: 1,
			Time: base.Add(time.Duration(i*50) * time.Millisecond),
		})
	}
	script, err := ToScript(events, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(script.Steps) != 2 || script.Steps[0].Clicks != 3 || script.Steps[1].Clicks != 1 {
		t.Fatalf("%+v", script.Steps)
	}
}

func TestRecorderSourceAndManualAdd(t *testing.T) {
	rec := New()
	base := time.Unix(10, 0)
	rec.Add(Event{Kind: KindClick, Button: core.ButtonLeft, X: 3, Y: 4, Time: base})
	rec.Add(Event{Kind: KindClick, Button: core.ButtonLeft, X: 8, Y: 9, Time: base.Add(time.Second)})
	script, err := ToScript(rec.Events(), Options{DoubleClickMs: -1, Name: "手動"})
	if err != nil {
		t.Fatal(err)
	}
	if len(script.Steps) != 2 || script.Steps[0].X != 3 || script.Steps[0].IntervalMs != 1000 {
		t.Fatalf("%+v", script.Steps)
	}

	src := &fakeSource{}
	live := New()
	if err := live.Start(nil); err == nil {
		t.Fatal("nil source")
	}
	if err := live.Start(src); err != nil {
		t.Fatal(err)
	}
	if err := live.Start(src); err == nil {
		t.Fatal("double start")
	}
	src.Emit(Event{Kind: KindClick, Button: core.ButtonRight, X: 2, Y: 2, Time: base})
	if !live.Active() {
		t.Fatal("not active")
	}
	live.Stop()
	live.Stop()
	if live.Active() {
		t.Fatal("still active")
	}
	src.Emit(Event{Kind: KindClick, Button: core.ButtonRight, X: 9, Y: 9, Time: base.Add(time.Second)})
	evs := live.Events()
	if len(evs) != 1 || evs[0].X != 2 {
		t.Fatalf("%+v", evs)
	}
	if _, err := ToScript(nil, Options{}); err == nil {
		t.Fatal("empty")
	}
}

type clickLog struct {
	mu  sync.Mutex
	log []string
}

func (c *clickLog) add(s string) {
	c.mu.Lock()
	c.log = append(c.log, s)
	c.mu.Unlock()
}

func (c *clickLog) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, len(c.log))
	copy(out, c.log)
	return out
}

func (c *clickLog) MoveTo(x, y int) error {
	c.add("move " + strconv.Itoa(x) + " " + strconv.Itoa(y))
	return nil
}

func (c *clickLog) Click(b core.Button, count, holdMs int) error {
	c.add("click " + string(b) + " " + strconv.Itoa(count) + " " + strconv.Itoa(holdMs))
	return nil
}

func (c *clickLog) sleep(_ context.Context, d time.Duration) error {
	if d > 0 {
		c.add("sleep " + strconv.FormatInt(d.Milliseconds(), 10))
	}
	return nil
}

type fakeSource struct {
	mu sync.Mutex
	fn func(Event)
}

func (f *fakeSource) Start(fn func(Event)) error {
	f.mu.Lock()
	f.fn = fn
	f.mu.Unlock()
	return nil
}

func (f *fakeSource) Stop() {
	f.mu.Lock()
	f.fn = nil
	f.mu.Unlock()
}

func (f *fakeSource) Emit(ev Event) {
	f.mu.Lock()
	fn := f.fn
	f.mu.Unlock()
	if fn != nil {
		fn(ev)
	}
}
