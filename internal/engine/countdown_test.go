package engine

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

type countdownOverlay struct {
	fakeOverlay
	cmu   sync.Mutex
	shown []int
}

func (c *countdownOverlay) ShowCountdown(n int) {
	c.cmu.Lock()
	c.shown = append(c.shown, n)
	c.cmu.Unlock()
}

func TestEngineCountdownEventsAndOverlay(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	e.Sleep = instant
	ov := &countdownOverlay{}
	e.SetOverlay(ov)
	e.SetShowClicks(false) // the countdown still shows
	s := &core.Script{Version: 1, Repeat: 1, Steps: []core.Step{{X: 3, Y: 4, Button: core.ButtonLeft}}}
	s.SetStartDelaySec(3)
	if err := e.Start(scriptJSON(t, s)); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	var kinds []string
	var msgs []string
	for {
		raw, ok := e.PollEvent()
		if !ok {
			break
		}
		var p eventPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		if p.Kind == string(core.EventCountdown) {
			kinds = append(kinds, p.Kind)
			msgs = append(msgs, p.Message)
			if p.Remaining != 3-len(msgs)+1 {
				t.Fatalf("remaining %d at #%d", p.Remaining, len(msgs))
			}
		}
	}
	if len(msgs) != 3 || msgs[0] != "3 秒後開始" || msgs[2] != "1 秒後開始" {
		t.Fatalf("countdown messages %v", msgs)
	}
	ov.cmu.Lock()
	defer ov.cmu.Unlock()
	want := []int{3, 2, 1, 0}
	if len(ov.shown) != len(want) {
		t.Fatalf("overlay countdown %v want %v", ov.shown, want)
	}
	for i := range want {
		if ov.shown[i] != want[i] {
			t.Fatalf("overlay countdown %v want %v", ov.shown, want)
		}
	}
}

func TestEngineStopDuringCountdownClicksNothing(t *testing.T) {
	cl := &fakeClicker{}
	e := New(cl, nil)
	entered := make(chan struct{}, 1)
	e.Sleep = func(ctx context.Context, d time.Duration) error {
		select {
		case entered <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}
	ov := &countdownOverlay{}
	e.SetOverlay(ov)
	// Field absent: the default 3 s countdown applies.
	raw := `{"version":1,"repeat":0,"steps":[{"x":9,"y":9,"button":"left"}]}`
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	<-entered
	e.Stop()
	_ = e.Wait()
	cl.mu.Lock()
	n := cl.clicks + cl.moves
	cl.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d clicks during countdown", n)
	}
	ov.cmu.Lock()
	defer ov.cmu.Unlock()
	if len(ov.shown) != 2 || ov.shown[0] != 3 || ov.shown[1] != 0 {
		t.Fatalf("overlay countdown %v want [3 0]", ov.shown)
	}
}
