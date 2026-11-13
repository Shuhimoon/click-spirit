package mobile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/imagematch"
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
	mu     sync.Mutex
	kinds  []string
	msgs   []string
	clicks [][5]int
}

func (f *fakeListener) OnClick(x, y, step, repetition, repeats int) {
	f.mu.Lock()
	f.clicks = append(f.clicks, [5]int{x, y, step, repetition, repeats})
	f.mu.Unlock()
}

type fakeOverlay struct {
	mu     sync.Mutex
	labels []string
	pts    [][2]int
}

func (f *fakeOverlay) ShowClick(x, y int, label string) {
	f.mu.Lock()
	f.pts = append(f.pts, [2]int{x, y})
	f.labels = append(f.labels, label)
	f.mu.Unlock()
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
	overlay = nil
	showOff = false
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
	EmergencyStop()
	if IsRunning() {
		t.Fatal("running")
	}
	_ = time.Millisecond
}

func TestStepRepeatAndCircle(t *testing.T) {
	reset(t)
	f := &fakeDisp{ok: true}
	l := &fakeListener{}
	SetDispatcher(f)
	SetListener(l)
	mu.Lock()
	e, err := ensureLocked()
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	e.Sleep = func(ctx context.Context, d time.Duration) error {
		return ctx.Err()
	}
	raw := mustJSON(t, &core.Script{
		Version: 1,
		Repeat:  2,
		Steps: []core.Step{
			{X: 1, Y: 1, Button: core.ButtonLeft, Repeat: 2},
			{X: 2, Y: 2, Button: core.ButtonLeft, Repeat: 3},
			{X: 3, Y: 3, Button: core.ButtonLeft},
			{Button: core.ButtonLeft, Repeat: 20, Target: &core.Target{Kind: core.TargetCircle, CX: 100, CY: 80, R: 40}},
		},
	})
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	got := append([]tap(nil), f.taps...)
	f.mu.Unlock()
	ordered := []tap{
		{1, 1, 0, 1}, {1, 1, 0, 1},
		{2, 2, 0, 1}, {2, 2, 0, 1}, {2, 2, 0, 1},
		{3, 3, 0, 1},
	}
	const circleN = 20
	perLoop := len(ordered) + circleN
	if len(got) != perLoop*2 {
		t.Fatalf("taps %d, want %d", len(got), perLoop*2)
	}
	var distinct bool
	var first *tap
	for loop := 0; loop < 2; loop++ {
		base := loop * perLoop
		for i, want := range ordered {
			if got[base+i] != want {
				t.Fatalf("loop %d tap[%d] = %#v want %#v", loop, i, got[base+i], want)
			}
		}
		for _, tp := range got[base+len(ordered) : base+perLoop] {
			if math.Hypot(float64(tp.x-100), float64(tp.y-80)) > 40.75 {
				t.Fatalf("outside disc %#v", tp)
			}
			if first == nil {
				cp := tp
				first = &cp
			} else if tp != *first {
				distinct = true
			}
		}
	}
	if !distinct {
		t.Fatal("circle samples were identical")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	joined := strings.Join(l.msgs, "\n")
	if !strings.Contains(joined, "#1") || !strings.Contains(joined, "#2") {
		t.Fatalf("messages %v", l.msgs)
	}
}

func TestEmergencyStop(t *testing.T) {
	reset(t)
	f := &fakeDisp{ok: true}
	l := &fakeListener{}
	SetDispatcher(f)
	SetListener(l)
	mu.Lock()
	e, err := ensureLocked()
	mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	e.Sleep = func(ctx context.Context, d time.Duration) error {
		select {
		case <-started:
		default:
			close(started)
		}
		<-ctx.Done()
		return ctx.Err()
	}
	raw := mustJSON(t, &core.Script{
		Version: 1,
		Repeat:  0,
		Steps:   []core.Step{{X: 1, Y: 1, Button: core.ButtonLeft, IntervalMs: 60_000}},
	})
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("script did not reach the interval")
	}
	EmergencyStop()
	if err := e.Wait(); !errors.Is(err, core.ErrStopped) {
		t.Fatal(err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if !strings.Contains(strings.Join(l.kinds, ","), "emergency_stop") {
		t.Fatalf("kinds %v", l.kinds)
	}
	if !strings.Contains(strings.Join(l.msgs, "\n"), "緊急停止") {
		t.Fatalf("messages %v", l.msgs)
	}
}

func TestMatchImageBytes(t *testing.T) {
	reset(t)
	rng := rand.New(rand.NewPCG(3, 4))
	screen := image.NewRGBA(image.Rect(0, 0, 48, 36))
	for y := 0; y < 36; y++ {
		for x := 0; x < 48; x++ {
			v := uint8(rng.IntN(200))
			screen.SetRGBA(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	templ := plusImage(7)
	const px, py = 11, 8
	for y := 0; y < 7; y++ {
		for x := 0; x < 7; x++ {
			screen.SetRGBA(px+x, py+y, templ.RGBAAt(x, y))
		}
	}
	raw := MatchImage(pngBytes(t, screen), pngBytes(t, templ), 0.9)
	var hit imagematch.Hit
	if err := json.Unmarshal([]byte(raw), &hit); err != nil {
		t.Fatal(err, raw)
	}
	if !hit.Found || hit.Error != "" || hit.X != px+7/2 || hit.Y != py+7/2 || hit.Score < 0.99 {
		t.Fatalf("%+v", hit)
	}

	other := checkerImage(7)
	miss := MatchImage(pngBytes(t, screen), pngBytes(t, other), 0)
	hit = imagematch.Hit{}
	if err := json.Unmarshal([]byte(miss), &hit); err != nil {
		t.Fatal(err, miss)
	}
	if hit.Found || hit.Score >= 0.9 {
		t.Fatalf("absent %+v", hit)
	}
	bad := MatchImage([]byte("nope"), pngBytes(t, templ), 0.9)
	if !strings.Contains(bad, `"found":false`) || !strings.Contains(bad, "error") {
		t.Fatal(bad)
	}
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func plusImage(n int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	mid := n / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if x == mid || y == mid {
				img.SetRGBA(x, y, color.RGBA{R: 255, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{A: 255})
			}
		}
	}
	return img
}

func checkerImage(n int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if (x+y)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{G: 255, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{B: 255, A: 255})
			}
		}
	}
	return img
}

func TestClickEventsReachListenerAndOverlay(t *testing.T) {
	reset(t)
	f := &fakeDisp{ok: true}
	l := &fakeListener{}
	ov := &fakeOverlay{}
	SetDispatcher(f)
	SetListener(l)
	SetOverlay(ov)
	raw := mustJSON(t, &core.Script{
		Version: 1,
		Repeat:  1,
		Steps: []core.Step{
			{X: 10, Y: 20, Button: core.ButtonLeft},
			{Button: core.ButtonLeft, Repeat: 3, Target: &core.Target{Kind: core.TargetCircle, CX: 200, CY: 300, R: 40}},
		},
	})
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	if err := currentEngine(t).Wait(); err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	taps := append([]tap(nil), f.taps...)
	f.mu.Unlock()
	l.mu.Lock()
	clicks := append([][5]int(nil), l.clicks...)
	l.mu.Unlock()
	ov.mu.Lock()
	labels := append([]string(nil), ov.labels...)
	pts := append([][2]int(nil), ov.pts...)
	ov.mu.Unlock()

	if len(clicks) != 4 || len(taps) != 4 || len(labels) != 4 {
		t.Fatalf("clicks %v taps %v labels %v", clicks, taps, labels)
	}
	wantLabels := []string{"步驟 1", "步驟 2 · 第 1/3 次", "步驟 2 · 第 2/3 次", "步驟 2 · 第 3/3 次"}
	for i, c := range clicks {
		if c[0] != taps[i].x || c[1] != taps[i].y {
			t.Fatalf("click %d at (%d,%d) but tap at (%d,%d)", i, c[0], c[1], taps[i].x, taps[i].y)
		}
		if pts[i] != [2]int{c[0], c[1]} || labels[i] != wantLabels[i] {
			t.Fatalf("overlay %d = %v %q, want %v %q", i, pts[i], labels[i], c[:2], wantLabels[i])
		}
		if i > 0 {
			if c[2] != 1 || c[3] != i-1 || c[4] != 3 {
				t.Fatalf("click %d = %v", i, c)
			}
			if d := math.Hypot(float64(c[0]-200), float64(c[1]-300)); d > 40.75 {
				t.Fatalf("circle tap %v outside r=40", c)
			}
		}
	}

	// Turning the overlay off keeps listener clicks but skips drawing.
	SetShowClicks(false)
	if ShowClicks() {
		t.Fatal("ShowClicks should be false")
	}
	if err := StartScript(raw); err != nil {
		t.Fatal(err)
	}
	if err := currentEngine(t).Wait(); err != nil {
		t.Fatal(err)
	}
	ov.mu.Lock()
	n := len(ov.labels)
	ov.mu.Unlock()
	if n != 4 {
		t.Fatalf("overlay drew while disabled: %d draws", n)
	}
}
