package mobile

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/internal/engine"
)

type fakeDisp struct {
	mu   sync.Mutex
	taps []tap
	ok   bool
}

type tap struct {
	x, y, duration, count int
}

func (f *fakeDisp) Tap(x, y, durationMs, count int) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.taps = append(f.taps, tap{x, y, durationMs, count})
	return f.ok
}

type fakeListener struct {
	mu    sync.Mutex
	kinds []string
	msgs  []string
}

func (f *fakeListener) OnEvent(kind string, loop, step int, message string) {
	f.mu.Lock()
	f.kinds = append(f.kinds, kind)
	f.msgs = append(f.msgs, message)
	f.mu.Unlock()
}

func reset(t *testing.T) {
	t.Helper()
	Stop()
	mu.Lock()
	e := eng
	mu.Unlock()
	if e != nil {
		_ = e.Wait()
	}
	mu.Lock()
	disp = nil
	listener = nil
	clicker = nil
	eng = nil
	mu.Unlock()
}

func TestSampleAndValidate(t *testing.T) {
	reset(t)
	raw := SampleScript()
	if !strings.Contains(raw, `"default_interval_ms"`) || !strings.Contains(raw, "示範腳本") {
		t.Fatalf("sample = %s", raw)
	}
	if err := ValidateScript(raw); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScript("{"); err == nil {
		t.Fatal("expected json error")
	}
	if err := ValidateScript(`{"version":1,"steps":[]}`); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestStartScriptTapsAndListener(t *testing.T) {
	reset(t)
	f := &fakeDisp{ok: true}
	l := &fakeListener{}
	SetDispatcher(f)
	SetListener(l)
	raw := mustJSON(t, &core.Script{
		Version: 1,
		Repeat:  1,
		Steps: []core.Step{
			{X: 10, Y: 20, Button: core.ButtonLeft, Clicks: 2, HoldMs: 40},
			{X: 30, Y: 40, Button: core.ButtonRight, Clicks: 0, HoldMs: 0},
		},
	})
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	if err := currentEngine(t).Wait(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	got := append([]tap(nil), f.taps...)
	f.mu.Unlock()
	want := []tap{{10, 20, 40, 2}, {30, 40, 0, 1}}
	if len(got) != len(want) {
		t.Fatalf("taps %#v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("tap[%d] = %#v want %#v", i, got[i], want[i])
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	joined := strings.Join(l.kinds, ",")
	if !strings.Contains(joined, "started") || !strings.Contains(joined, "finished") {
		t.Fatalf("kinds %v", l.kinds)
	}
	if len(l.msgs) == 0 || l.msgs[0] == "" {
		t.Fatalf("messages %v", l.msgs)
	}
}

func TestStartRequiresDispatcherAndRejectsDoubleStart(t *testing.T) {
	reset(t)
	raw := mustJSON(t, &core.Script{
		Version:      1,
		Repeat:       0,
		StartDelayMs: 60_000,
		Steps:        []core.Step{{X: 1, Y: 1, Button: core.ButtonLeft, Clicks: 1}},
	})
	if err := StartScript(raw); err == nil {
		t.Fatal("expected missing dispatcher")
	}
	f := &fakeDisp{ok: true}
	SetDispatcher(f)
	mu.Lock()
	e, err := ensureLocked()
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	e.Sleep = func(ctx context.Context, d time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	if err := StartScript(raw); !errors.Is(err, core.ErrAlreadyRunning) {
		t.Fatalf("second start %v", err)
	}
	if !IsRunning() {
		t.Fatal("expected running")
	}
	Stop()
	if err := e.Wait(); !errors.Is(err, core.ErrStopped) {
		t.Fatal(err)
	}
	if IsRunning() {
		t.Fatal("still running")
	}
}

func TestDispatchFailure(t *testing.T) {
	reset(t)
	f := &fakeDisp{ok: false}
	SetDispatcher(f)
	raw := mustJSON(t, &core.Script{
		Version: 1,
		Repeat:  1,
		Steps:   []core.Step{{X: 1, Y: 2, Button: core.ButtonMiddle, Clicks: 1}},
	})
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	err := currentEngine(t).Wait()
	if err == nil || !strings.Contains(err.Error(), "gesture") {
		t.Fatalf("got %v", err)
	}
}

func currentEngine(t *testing.T) *engine.Engine {
	t.Helper()
	mu.Lock()
	defer mu.Unlock()
	if eng == nil {
		t.Fatal("nil engine")
	}
	return eng
}

func mustJSON(t *testing.T, s *core.Script) string {
	t.Helper()
	b, err := s.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestStopWithoutEngine(t *testing.T) {
	reset(t)
	Stop()
	if IsRunning() {
		t.Fatal("running")
	}
	_ = time.Millisecond
}
