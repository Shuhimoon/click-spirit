package engine

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

type fakeClicker struct {
	mu     sync.Mutex
	moves  int
	clicks int
	err    error
	x, y   int
	locErr error
	hasLoc bool
}

func (f *fakeClicker) MoveTo(x, y int) error {
	f.mu.Lock()
	f.moves++
	f.x, f.y = x, y
	f.mu.Unlock()
	return nil
}

func (f *fakeClicker) Click(b core.Button, count, holdMs int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.clicks += count
	return nil
}

func (f *fakeClicker) CursorPos() (int, int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hasLoc {
		return 0, 0, errors.New("no cursor")
	}
	return f.x, f.y, f.locErr
}

func instant(context.Context, time.Duration) error { return nil }

func scriptJSON(t *testing.T, s *core.Script) string {
	t.Helper()
	b, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestEngineRunAndCursor(t *testing.T) {
	f := &fakeClicker{x: 4, y: 5, hasLoc: true}
	e := New(f, nil)
	e.Sleep = instant
	raw := scriptJSON(t, &core.Script{
		Version: 1,
		Repeat:  2,
		Steps: []core.Step{
			{X: 4, Y: 5, Button: core.ButtonLeft, Clicks: 2},
		},
	})
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	if e.Running() {
		t.Fatal("still running")
	}
	f.mu.Lock()
	moves, clicks := f.moves, f.clicks
	f.mu.Unlock()
	if moves != 2 || clicks != 4 {
		t.Fatalf("moves %d clicks %d", moves, clicks)
	}
	x, y, ok := e.CursorPos()
	if !ok || x != 4 || y != 5 {
		t.Fatalf("cursor %d %d ok=%v", x, y, ok)
	}

	kinds := pollKinds(t, e)
	if len(kinds) == 0 || kinds[0] != "started" || kinds[len(kinds)-1] != "finished" {
		t.Fatalf("kinds %v", kinds)
	}
}

func TestEngineUnsupportedCursorAndInitError(t *testing.T) {
	f := &fakeClicker{}
	e := New(f, nil)
	if _, _, ok := e.CursorPos(); ok {
		t.Fatal("cursor should be unsupported")
	}
	bad := New(nil, errors.New("no desktop"))
	if err := bad.Start(scriptJSON(t, core.NewSample())); err == nil || err.Error() != "no desktop" {
		t.Fatalf("got %v", err)
	}
}

func TestEngineClickerErrorAndBadJSON(t *testing.T) {
	boom := errors.New("boom")
	f := &fakeClicker{err: boom}
	e := New(f, nil)
	e.Sleep = instant
	raw := scriptJSON(t, &core.Script{
		Version: 1,
		Repeat:  1,
		Steps:   []core.Step{{X: 1, Y: 2, Button: core.ButtonRight, Clicks: 1}},
	})
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	err := e.Wait()
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	kinds := pollKinds(t, e)
	if kinds[len(kinds)-1] != "error" {
		t.Fatalf("kinds %v", kinds)
	}
	if err := e.Start("{"); err == nil {
		t.Fatal("expected json error")
	}
}

func TestEngineStopAndDoubleStart(t *testing.T) {
	f := &fakeClicker{}
	e := New(f, nil)
	e.Sleep = func(ctx context.Context, d time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}
	raw := scriptJSON(t, &core.Script{
		Version:      1,
		Repeat:       0,
		StartDelayMs: 60_000,
		Steps:        []core.Step{{X: 1, Y: 1, Button: core.ButtonLeft, Clicks: 1}},
	})
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	if err := e.Start(raw); !errors.Is(err, core.ErrAlreadyRunning) {
		t.Fatalf("second start: %v", err)
	}
	e.Stop()
	err := waitEngine(t, e)
	if !errors.Is(err, core.ErrStopped) {
		t.Fatal(err)
	}
}

func TestEngineDropsOldest(t *testing.T) {
	f := &fakeClicker{}
	e := New(f, nil)
	e.maxQueue = 2
	e.Sleep = instant
	raw := scriptJSON(t, &core.Script{
		Version: 1,
		Repeat:  1,
		Steps:   []core.Step{{X: 1, Y: 1, Button: core.ButtonLeft, Clicks: 1}},
	})
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	kinds := pollKinds(t, e)
	if len(kinds) != 2 || kinds[0] != "loopDone" || kinds[1] != "finished" {
		t.Fatalf("kinds %v", kinds)
	}
	if _, ok := e.PollEvent(); ok {
		t.Fatal("queue not empty")
	}
}

func pollKinds(t *testing.T, e *Engine) []string {
	t.Helper()
	var kinds []string
	for {
		s, ok := e.PollEvent()
		if !ok {
			return kinds
		}
		var p eventPayload
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			t.Fatal(err)
		}
		kinds = append(kinds, p.Kind)
	}
}

func waitEngine(t *testing.T, e *Engine) error {
	t.Helper()
	ch := make(chan error, 1)
	go func() { ch <- e.Wait() }()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	return nil
}
