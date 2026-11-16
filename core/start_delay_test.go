package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestStartDelayDefaultsToThreeWhenFieldAbsent(t *testing.T) {
	s, err := Unmarshal([]byte(`{"version":1,"steps":[{"x":1,"y":2,"button":"left"}],"repeat":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if s.StartDelaySec != nil || s.EffectiveStartDelaySec() != 3 {
		t.Fatalf("missing field: ptr=%v eff=%d", s.StartDelaySec, s.EffectiveStartDelaySec())
	}
	s, _ = Unmarshal([]byte(`{"version":1,"steps":[{"x":1,"y":2,"button":"left"}],"start_delay_s":0}`))
	if s.EffectiveStartDelaySec() != 0 {
		t.Fatalf("explicit 0 = %d", s.EffectiveStartDelaySec())
	}
	s.SetStartDelaySec(7)
	b, _ := s.Marshal()
	back, _ := Unmarshal(b)
	if back.EffectiveStartDelaySec() != 7 {
		t.Fatalf("round trip = %d (%s)", back.EffectiveStartDelaySec(), b)
	}
	for _, bad := range []int{-1, MaxStartDelaySec + 1} {
		s.SetStartDelaySec(bad)
		if s.Validate() == nil {
			t.Fatalf("start_delay_s %d accepted", bad)
		}
	}
}

// fakeClock is an injectable clock: Sleep advances virtual time at once.
type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	return nil
}

type timedClicker struct {
	clock *fakeClock
	mu    sync.Mutex
	at    []time.Time
}

func (c *timedClicker) MoveTo(x, y int) error { return nil }
func (c *timedClicker) Click(b Button, n, hold int) error {
	c.mu.Lock()
	c.at = append(c.at, c.clock.Now())
	c.mu.Unlock()
	return nil
}

func TestStartDelayIsHonored(t *testing.T) {
	for _, sec := range []int{0, 3, 5} {
		start := time.Date(2026, 11, 16, 21, 0, 0, 0, time.UTC)
		clock := &fakeClock{now: start}
		cl := &timedClicker{clock: clock}
		var remaining []int
		r := &Runner{Clicker: cl, Sleep: clock.Sleep, Now: clock.Now, OnEvent: func(ev Event) {
			if ev.Kind == EventCountdown {
				remaining = append(remaining, ev.Remaining)
			}
		}}
		s := &Script{Version: 1, Repeat: 1, Steps: []Step{{X: 1, Y: 1, Button: ButtonLeft}}}
		s.SetStartDelaySec(sec)
		if err := r.Run(context.Background(), s); err != nil {
			t.Fatal(err)
		}
		if len(cl.at) != 1 {
			t.Fatalf("delay %d: %d clicks", sec, len(cl.at))
		}
		if got := cl.at[0].Sub(start); got != time.Duration(sec)*time.Second {
			t.Fatalf("delay %d: first click after %v", sec, got)
		}
		if len(remaining) != sec {
			t.Fatalf("delay %d: countdown events %v", sec, remaining)
		}
		for i, v := range remaining {
			if v != sec-i {
				t.Fatalf("delay %d: countdown %v not descending", sec, remaining)
			}
		}
	}
}

func TestStartDelayCancelProducesNoClicks(t *testing.T) {
	rec := &recorder{}
	tick := make(chan struct{})
	var events []EventKind
	var mu sync.Mutex
	r := &Runner{
		Clicker: rec,
		// Block on the first countdown second until the test cancels.
		Sleep: func(ctx context.Context, d time.Duration) error {
			close(tick)
			<-ctx.Done()
			return ctx.Err()
		},
		OnEvent: func(ev Event) {
			mu.Lock()
			events = append(events, ev.Kind)
			mu.Unlock()
		},
	}
	s := &Script{Version: 1, Repeat: 0, Steps: []Step{{X: 5, Y: 6, Button: ButtonLeft}}}
	if s.EffectiveStartDelaySec() != DefaultStartDelaySec {
		t.Fatal("literal script should use the default countdown")
	}
	if err := r.Start(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	<-tick
	r.Stop() // the emergency hotkey calls the same Stop
	if err := r.Wait(); !errors.Is(err, ErrStopped) {
		t.Fatalf("err = %v", err)
	}
	if got := rec.snapshot(); len(got) != 0 {
		t.Fatalf("clicks during countdown: %v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []EventKind{EventStarted, EventCountdown, EventStopped}
	if len(events) != len(want) {
		t.Fatalf("events %v", events)
	}
	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events %v want %v", events, want)
		}
	}
}

func TestRunnerExcludeSkipsBubbleClicks(t *testing.T) {
	rec := &recorder{}
	bubble := Rect{X: 90, Y: 90, W: 20, H: 20}
	var kinds []EventKind
	r := &Runner{Clicker: rec, Sleep: rec.sleep, Exclude: func(x, y int) bool { return bubble.Contains(x, y) },
		OnEvent: func(ev Event) { kinds = append(kinds, ev.Kind) }}
	s := &Script{StartDelaySec: StartDelay(0), Version: 1, Repeat: 1, Steps: []Step{
		{X: 100, Y: 100, Button: ButtonLeft},
		{X: 5, Y: 6, Button: ButtonLeft},
	}}
	if err := r.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	got := rec.snapshot()
	if len(got) != 2 || got[0] != "move 5 6" {
		t.Fatalf("clicks %v", got)
	}
	found := false
	for _, k := range kinds {
		if k == EventExcluded {
			found = true
		}
	}
	if !found {
		t.Fatalf("no excluded event in %v", kinds)
	}
}
