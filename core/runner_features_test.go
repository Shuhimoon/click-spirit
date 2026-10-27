package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunnerStepRepeatOrder(t *testing.T) {
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
		Repeat:            2,
		DefaultIntervalMs: 999,
		LoopIntervalMs:    40,
		Steps: []Step{
			{X: 1, Y: 1, Button: ButtonLeft, Clicks: 1, IntervalMs: 10, Repeat: 2},
			{X: 2, Y: 2, Button: ButtonRight, Clicks: 1, IntervalMs: 20, Repeat: 3},
			{X: 3, Y: 3, Button: ButtonMiddle, Clicks: 1, IntervalMs: 30, Repeat: 0},
		},
	}
	if err := runner.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	one := []string{
		"move 1 1", "click left 1 0", "sleep 10",
		"move 1 1", "click left 1 0", "sleep 10",
		"move 2 2", "click right 1 0", "sleep 20",
		"move 2 2", "click right 1 0", "sleep 20",
		"move 2 2", "click right 1 0", "sleep 20",
		"move 3 3", "click middle 1 0", "sleep 30",
	}
	want := append(append([]string{}, one...), "sleep 40")
	want = append(want, one...)
	got := rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("len %d want %d\n%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%q want %q\n%v", i, got[i], want[i], got)
		}
	}
	evMu.Lock()
	defer evMu.Unlock()
	type pair struct{ step, rep int }
	var gotPairs []pair
	for _, ev := range events {
		if ev.Kind == EventStep {
			gotPairs = append(gotPairs, pair{ev.StepIndex, ev.Repetition})
		}
	}
	wantPairs := []pair{
		{0, 0}, {0, 1}, {1, 0}, {1, 1}, {1, 2}, {2, 0},
		{0, 0}, {0, 1}, {1, 0}, {1, 1}, {1, 2}, {2, 0},
	}
	if len(gotPairs) != len(wantPairs) {
		t.Fatalf("events %+v", gotPairs)
	}
	for i := range wantPairs {
		if gotPairs[i] != wantPairs[i] {
			t.Fatalf("pair %d %+v", i, gotPairs)
		}
	}
}

func TestRunnerPerStepIntervalUsesDefault(t *testing.T) {
	rec := &recorder{}
	runner := &Runner{Clicker: rec, Sleep: rec.sleep}
	script := &Script{
		Version:           1,
		Repeat:            1,
		DefaultIntervalMs: 40,
		Steps: []Step{
			{X: 1, Y: 1, Button: ButtonLeft, Clicks: 1, IntervalMs: 0, Repeat: 2},
			{X: 2, Y: 2, Button: ButtonRight, Clicks: 1, IntervalMs: 15, Repeat: 1},
		},
	}
	if err := runner.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"move 1 1", "click left 1 0", "sleep 40",
		"move 1 1", "click left 1 0", "sleep 40",
		"move 2 2", "click right 1 0", "sleep 15",
	}
	got := rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestRunnerStopMidSequence(t *testing.T) {
	rec := &recorder{}
	hit := make(chan struct{}, 1)
	runner := &Runner{
		Clicker: &signalClicker{recorder: rec, hit: hit},
		Sleep: func(ctx context.Context, d time.Duration) error {
			rec.add("sleep")
			<-ctx.Done()
			return ctx.Err()
		},
	}
	script := &Script{
		Version: 1,
		Repeat:  1,
		Steps: []Step{
			{X: 1, Y: 1, Button: ButtonLeft, Clicks: 1, IntervalMs: 10, Repeat: 3},
			{X: 9, Y: 9, Button: ButtonRight, Clicks: 1, IntervalMs: 10, Repeat: 1},
		},
	}
	if err := runner.Start(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	select {
	case <-hit:
	case <-time.After(2 * time.Second):
		t.Fatal("no click")
	}
	runner.Stop()
	err := waitErr(t, runner)
	if !errors.Is(err, ErrStopped) {
		t.Fatal(err)
	}
	got := rec.snapshot()
	for _, line := range got {
		if line == "move 9 9" || line == "click right 1 0" {
			t.Fatalf("continued after stop: %v", got)
		}
	}
	clicks := 0
	for _, line := range got {
		if len(line) >= 5 && line[:5] == "click" {
			clicks++
		}
	}
	if clicks != 1 {
		t.Fatalf("clicks %d log %v", clicks, got)
	}
}

func TestRunnerCircleIsDeterministic(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 99))
	probe := rand.New(rand.NewPCG(42, 99))
	wx, wy := PointInDisc(50, 60, 15, probe.Float64(), probe.Float64())
	rec := &recorder{}
	runner := &Runner{Clicker: rec, Sleep: rec.sleep, Rand: rng.Float64}
	script := &Script{
		Version: 1,
		Repeat:  1,
		Steps: []Step{{
			Button: ButtonLeft, Clicks: 1, IntervalMs: 1,
			Target: &Target{Kind: TargetCircle, CX: 50, CY: 60, R: 15},
		}},
	}
	if err := runner.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	got := rec.snapshot()
	wantMove := "move " + itoa(wx) + " " + itoa(wy)
	if len(got) == 0 || got[0] != wantMove {
		t.Fatalf("got %v want %s", got, wantMove)
	}
}

func TestOldScriptRunsIdentically(t *testing.T) {
	const raw = `{
  "version": 1,
  "name": "示範腳本",
  "steps": [
    {"x": 100, "y": 200, "button": "left", "clicks": 1, "interval_ms": 50, "hold_ms": 0},
    {"x": 300, "y": 400, "button": "right", "clicks": 2, "interval_ms": 0, "hold_ms": 30}
  ],
  "repeat": 1,
  "default_interval_ms": 100,
  "loop_interval_ms": 500,
  "start_delay_ms": 200
}`
	s, err := Unmarshal([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	runner := &Runner{Clicker: rec, Sleep: rec.sleep}
	if err := runner.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"sleep 200",
		"move 100 200", "click left 1 0", "sleep 50",
		"move 300 400", "click right 2 30", "sleep 100",
	}
	got := rec.snapshot()
	if len(got) != len(want) {
		t.Fatalf("%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestRunnerImageClickSkipStopWait(t *testing.T) {
	screen, templ := scene(t)
	miss := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for i := 0; i < len(miss.Pix); i += 4 {
		miss.Pix[i] = 20
		miss.Pix[i+1] = 20
		miss.Pix[i+2] = 20
		miss.Pix[i+3] = 255
	}
	b64 := pngBase64(t, templ)

	t.Run("center plus offset", func(t *testing.T) {
		rec := &recorder{}
		runner := &Runner{Clicker: rec, Sleep: rec.sleep, Capturer: stillCap{img: screen}}
		script := imageScript(b64, OnNotFoundStop, 0, 0)
		script.Steps[0].Target.OffsetX = 2
		script.Steps[0].Target.OffsetY = -1
		script.Steps[0].Target.Region = &Region{X: 0, Y: 0, W: 40, H: 30}
		if err := runner.Run(context.Background(), script); err != nil {
			t.Fatal(err)
		}
		// Paste at (8, 6), template is 5x5, center is +2, offset (2, -1) → (12, 7).
		got := rec.snapshot()
		if len(got) < 2 || got[0] != "move 12 7" || got[1] != "click left 1 0" {
			t.Fatalf("%v", got)
		}
	})

	t.Run("file template", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "t.png")
		if err := os.WriteFile(path, pngBytes(t, templ), 0o644); err != nil {
			t.Fatal(err)
		}
		rec := &recorder{}
		runner := &Runner{Clicker: rec, Sleep: rec.sleep, Capturer: stillCap{img: screen}}
		script := &Script{
			Version: 1, Repeat: 1,
			Steps: []Step{{
				Button: ButtonLeft, Clicks: 1,
				Target: &Target{Kind: TargetImage, Template: path, OnNotFound: OnNotFoundStop},
			}},
		}
		if err := runner.Run(context.Background(), script); err != nil {
			t.Fatal(err)
		}
		if rec.snapshot()[0] != "move 10 8" {
			t.Fatal(rec.snapshot())
		}
	})

	t.Run("skip", func(t *testing.T) {
		rec := &recorder{}
		runner := &Runner{Clicker: rec, Sleep: rec.sleep, Capturer: stillCap{img: miss}}
		script := imageScript(b64, OnNotFoundSkip, 0, 0)
		script.Steps[0].IntervalMs = 7
		script.Steps = append(script.Steps, Step{X: 1, Y: 2, Button: ButtonLeft, Clicks: 1, IntervalMs: 1})
		if err := runner.Run(context.Background(), script); err != nil {
			t.Fatal(err)
		}
		got := rec.snapshot()
		want := []string{"sleep 7", "move 1 2", "click left 1 0", "sleep 1"}
		if len(got) != len(want) {
			t.Fatalf("%v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%v", got)
			}
		}
	})

	t.Run("stop", func(t *testing.T) {
		rec := &recorder{}
		var kinds []EventKind
		runner := &Runner{
			Clicker: rec, Sleep: rec.sleep, Capturer: stillCap{img: miss},
			OnEvent: func(ev Event) { kinds = append(kinds, ev.Kind) },
		}
		script := imageScript(b64, OnNotFoundStop, 0, 0)
		script.Steps = append(script.Steps, Step{X: 4, Y: 5, Button: ButtonLeft, Clicks: 1})
		err := runner.Run(context.Background(), script)
		if !errors.Is(err, ErrImageNotFound) {
			t.Fatal(err)
		}
		if len(rec.snapshot()) != 0 {
			t.Fatal(rec.snapshot())
		}
		if kinds[len(kinds)-1] != EventError {
			t.Fatalf("%v", kinds)
		}
	})

	t.Run("nil capturer", func(t *testing.T) {
		runner := &Runner{Clicker: &recorder{}, Sleep: func(context.Context, time.Duration) error { return nil }}
		err := runner.Run(context.Background(), imageScript(b64, OnNotFoundSkip, 0, 0))
		if !errors.Is(err, ErrNoCapturer) {
			t.Fatal(err)
		}
	})

	t.Run("wait then click", func(t *testing.T) {
		rec := &recorder{}
		var mu sync.Mutex
		clock := time.Unix(0, 0)
		runner := &Runner{
			Clicker:  rec,
			Capturer: &seqCap{imgs: []image.Image{miss, miss, screen}},
			Now: func() time.Time {
				mu.Lock()
				defer mu.Unlock()
				return clock
			},
			Sleep: func(ctx context.Context, d time.Duration) error {
				mu.Lock()
				clock = clock.Add(d)
				mu.Unlock()
				rec.add("sleep " + itoa(int(d.Milliseconds())))
				return nil
			},
		}
		script := imageScript(b64, OnNotFoundWait, 500, 40)
		if err := runner.Run(context.Background(), script); err != nil {
			t.Fatal(err)
		}
		got := rec.snapshot()
		want := []string{"sleep 40", "sleep 40", "move 10 8", "click left 1 0"}
		if len(got) != len(want) {
			t.Fatalf("%v", got)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%v", got)
			}
		}
	})

	t.Run("wait timeout", func(t *testing.T) {
		rec := &recorder{}
		var mu sync.Mutex
		clock := time.Unix(0, 0)
		runner := &Runner{
			Clicker:  rec,
			Capturer: stillCap{img: miss},
			Now: func() time.Time {
				mu.Lock()
				defer mu.Unlock()
				return clock
			},
			Sleep: func(ctx context.Context, d time.Duration) error {
				mu.Lock()
				clock = clock.Add(d)
				mu.Unlock()
				rec.add("sleep")
				return nil
			},
		}
		err := runner.Run(context.Background(), imageScript(b64, OnNotFoundWait, 100, 40))
		if !errors.Is(err, ErrImageNotFound) {
			t.Fatal(err)
		}
		if len(rec.snapshot()) != 3 {
			t.Fatalf("%v", rec.snapshot())
		}
	})
}

func TestRunnerBadTemplate(t *testing.T) {
	runner := &Runner{Clicker: &recorder{}}
	script := &Script{
		Version: 1, Repeat: 1,
		Steps: []Step{{
			Button: ButtonLeft, Clicks: 1,
			Target: &Target{Kind: TargetImage, TemplateB64: "!!!!"},
		}},
	}
	if err := runner.Run(context.Background(), script); err == nil {
		t.Fatal("expected decode error")
	}
}

type signalClicker struct {
	*recorder
	hit chan struct{}
}

func (s *signalClicker) Click(b Button, count, holdMs int) error {
	err := s.recorder.Click(b, count, holdMs)
	select {
	case s.hit <- struct{}{}:
	default:
	}
	return err
}

type stillCap struct {
	img image.Image
	err error
}

func (s stillCap) Capture() (image.Image, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.img, nil
}

type seqCap struct {
	mu   sync.Mutex
	imgs []image.Image
	i    int
}

func (s *seqCap) Capture() (image.Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	img := s.imgs[s.i]
	if s.i+1 < len(s.imgs) {
		s.i++
	}
	return img, nil
}

func imageScript(b64, onNotFound string, timeout, poll int) *Script {
	return &Script{
		Version: 1,
		Repeat:  1,
		Steps: []Step{{
			Button: ButtonLeft,
			Clicks: 1,
			Target: &Target{
				Kind:        TargetImage,
				TemplateB64: b64,
				Threshold:   0.9,
				OnNotFound:  onNotFound,
				TimeoutMs:   timeout,
				PollMs:      poll,
			},
		}},
	}
}

func scene(t *testing.T) (screen, templ *image.RGBA) {
	t.Helper()
	templ = image.NewRGBA(image.Rect(0, 0, 5, 5))
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			v := uint8(0)
			if x == 2 || y == 2 {
				v = 255
			}
			templ.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	screen = image.NewRGBA(image.Rect(0, 0, 40, 30))
	for i := 0; i < len(screen.Pix); i += 4 {
		screen.Pix[i] = 30
		screen.Pix[i+1] = 30
		screen.Pix[i+2] = 30
		screen.Pix[i+3] = 255
	}
	for y := 0; y < 5; y++ {
		for x := 0; x < 5; x++ {
			screen.SetRGBA(8+x, 6+y, templ.RGBAAt(x, y))
		}
	}
	return screen, templ
}

func pngBase64(t *testing.T, img image.Image) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString(pngBytes(t, img))
}

func pngBytes(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
