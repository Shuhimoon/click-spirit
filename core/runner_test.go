package core

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"
)

type recorder struct {
	mu       sync.Mutex
	log      []string
	clickErr error
}

func (r *recorder) add(s string) {
	r.mu.Lock()
	r.log = append(r.log, s)
	r.mu.Unlock()
}

func (r *recorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.log))
	copy(out, r.log)
	return out
}

func (r *recorder) MoveTo(x, y int) error {
	r.add("move " + strconv.Itoa(x) + " " + strconv.Itoa(y))
	return nil
}

func (r *recorder) Click(b Button, count, holdMs int) error {
	r.mu.Lock()
	err := r.clickErr
	r.mu.Unlock()
	if err != nil {
		return err
	}
	r.add("click " + string(b) + " " + strconv.Itoa(count) + " " + strconv.Itoa(holdMs))
	return nil
}

func (r *recorder) sleep(ctx context.Context, d time.Duration) error {
	r.add("sleep " + strconv.FormatInt(d.Milliseconds(), 10))
	return nil
}

func TestRunnerRepeatOrder(t *testing.T) {
	rec := &recorder{}
	var events []Event
	var evMu sync.Mutex
	runner := &Runner{
		Clicker: rec,
		Sleep:   rec.sleep,
		OnEvent: func(ev Event) {
			evMu.Lock()
			events = append(events, ev)
			evMu.Unlock()
		},
	}
	script := &Script{
		Version:           1,
		Name:              "order",
		Repeat:            3,
		DefaultIntervalMs: 10,
		LoopIntervalMs:    20,
		StartDelayMs:      5,
		Steps: []Step{
			{X: 1, Y: 2, Button: ButtonLeft, Clicks: 0, IntervalMs: 0, HoldMs: 7},
			{X: 3, Y: 4, Button: ButtonRight, Clicks: 2, IntervalMs: 15, HoldMs: 0},
		},
	}
	if err := runner.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sleep 5",
		"move 1 2", "click left 1 7", "sleep 10",
		"move 3 4", "click right 2 0", "sleep 15",
		"sleep 20",
		"move 1 2", "click left 1 7", "sleep 10",
		"move 3 4", "click right 2 0", "sleep 15",
		"sleep 20",
		"move 1 2", "click left 1 7", "sleep 10",
		"move 3 4", "click right 2 0", "sleep 15",
	}
	got := rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("log len %d want %d\n%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("log[%d]=%q want %q\nfull %v", i, got[i], want[i], got)
		}
	}
	evMu.Lock()
	defer evMu.Unlock()
	var steps, loops int
	if len(events) == 0 || events[0].Kind != EventStarted {
		t.Fatalf("first event %#v", events)
	}
	if events[len(events)-1].Kind != EventFinished {
		t.Fatalf("last event %#v", events[len(events)-1])
	}
	for _, ev := range events {
		switch ev.Kind {
		case EventStep:
			steps++
		case EventLoopDone:
			loops++
		}
	}
	if steps != 6 || loops != 3 {
		t.Fatalf("steps %d loops %d", steps, loops)
	}
}

func TestRunnerStopInfinite(t *testing.T) {
	rec := &recorder{}
	runner := &Runner{
		Clicker: rec,
		Sleep: func(ctx context.Context, d time.Duration) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	script := &Script{
		Version:      1,
		Repeat:       0,
		StartDelayMs: 60_000,
		Steps:        []Step{{X: 1, Y: 1, Button: ButtonLeft, Clicks: 1}},
	}
	if err := runner.Start(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	runner.Stop()
	err := waitErr(t, runner)
	if !errors.Is(err, ErrStopped) || !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
	if runner.Running() {
		t.Fatal("still running")
	}
}

func TestRunnerStartTwice(t *testing.T) {
	runner := &Runner{
		Clicker: &recorder{},
		Sleep: func(ctx context.Context, d time.Duration) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	script := &Script{
		Version:      1,
		Repeat:       0,
		StartDelayMs: 60_000,
		Steps:        []Step{{X: 0, Y: 0, Button: ButtonMiddle, Clicks: 1}},
	}
	if err := runner.Start(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	err := runner.Start(context.Background(), script)
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("got %v", err)
	}
	runner.Stop()
	if err := waitErr(t, runner); !errors.Is(err, ErrStopped) {
		t.Fatal(err)
	}
}

func TestRunnerClickerError(t *testing.T) {
	boom := errors.New("boom")
	rec := &recorder{clickErr: boom}
	var kinds []EventKind
	runner := &Runner{
		Clicker: rec,
		OnEvent: func(ev Event) { kinds = append(kinds, ev.Kind) },
	}
	script := &Script{
		Version: 1,
		Repeat:  1,
		Steps:   []Step{{X: 8, Y: 9, Button: ButtonLeft, Clicks: 1, HoldMs: 5}},
	}
	err := runner.Run(context.Background(), script)
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	if len(kinds) == 0 || kinds[len(kinds)-1] != EventError {
		t.Fatalf("kinds %v", kinds)
	}
	for _, k := range kinds {
		if k == EventFinished {
			t.Fatal("finished after error")
		}
	}
}

func TestRunnerRejectsInvalidAndNilClicker(t *testing.T) {
	runner := &Runner{Clicker: &recorder{}}
	if err := runner.Run(context.Background(), &Script{}); err == nil {
		t.Fatal("expected validation error")
	}
	runner.Clicker = nil
	if err := runner.Start(context.Background(), NewSample()); err == nil {
		t.Fatal("expected nil clicker error")
	}
}

func waitErr(t *testing.T, r *Runner) error {
	t.Helper()
	ch := make(chan error, 1)
	go func() { ch <- r.Wait() }()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for runner")
	}
	return nil
}
