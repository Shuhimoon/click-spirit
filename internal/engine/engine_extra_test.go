package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

func TestEngineStepRepetitionAndHotkeyFieldIgnored(t *testing.T) {
	f := &fakeClicker{}
	e := New(f, nil)
	e.Sleep = instant
	raw := scriptJSON(t, &core.Script{
		Version:         1,
		Repeat:          1,
		EmergencyHotkey: "F12",
		Steps: []core.Step{
			{X: 3, Y: 4, Button: core.ButtonLeft, Clicks: 1, Repeat: 2},
		},
	})
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	var reps []int
	for {
		s, ok := e.PollEvent()
		if !ok {
			break
		}
		var p struct {
			Kind       string `json:"kind"`
			Step       int    `json:"step"`
			Repetition int    `json:"repetition"`
			Message    string `json:"message"`
		}
		if err := json.Unmarshal([]byte(s), &p); err != nil {
			t.Fatal(err)
		}
		if p.Kind == "step" {
			reps = append(reps, p.Repetition)
			if p.Step != 0 || !strings.Contains(p.Message, "#") {
				t.Fatalf("%+v %s", p, s)
			}
		}
	}
	if len(reps) != 2 || reps[0] != 0 || reps[1] != 1 {
		t.Fatal(reps)
	}
}

func TestEngineEmergencyHotkey(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	e.Sleep = func(ctx context.Context, d time.Duration) error {
		<-ctx.Done()
		return ctx.Err()
	}
	hk := &fakeKey{}
	e.SetHotkeyFactory(func(name string) (core.HotkeySource, error) {
		if name != core.DefaultHotkey {
			return nil, errors.New(name)
		}
		return hk, nil
	})
	if err := e.SetHotkey("not a key"); err == nil {
		t.Fatal("expected parse error")
	}
	if err := e.SetHotkey(core.DefaultHotkey); err != nil {
		t.Fatal(err)
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
	waitKind(t, e, "started")
	started := time.Now()
	hk.trigger()
	err := waitEngine(t, e)
	if time.Since(started) > time.Second {
		t.Fatal("stop was late")
	}
	if !errors.Is(err, core.ErrStopped) {
		t.Fatal(err)
	}
	if !hasKind(pollKinds(t, e), "emergency_stop") {
		t.Fatal("missing emergency_stop")
	}
	if err := e.SetHotkey(""); err != nil {
		t.Fatal(err)
	}
}

func TestEngineRecording(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	if err := e.StartRecording(); err == nil {
		t.Fatal("expected missing factory")
	}
	src := &fakeInput{}
	e.SetInputFactory(func() (record.InputSource, error) { return src, nil })
	if err := e.StartRecording(); err != nil {
		t.Fatal(err)
	}
	base := time.Unix(0, 0)
	src.emit(record.Event{Kind: record.KindClick, Button: core.ButtonLeft, X: 4, Y: 5, Time: base})
	src.emit(record.Event{Kind: record.KindClick, Button: core.ButtonLeft, X: 4, Y: 6, Time: base.Add(time.Second)})
	e.SetRecordOptions(record.Options{DoubleClickMs: -1})
	raw, err := e.StopRecording()
	if err != nil {
		t.Fatal(err)
	}
	script, err := core.Unmarshal([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if err := script.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(script.Steps) != 2 || script.Steps[0].X != 4 || script.Steps[0].IntervalMs != 1000 {
		t.Fatalf("%+v", script.Steps)
	}
	if _, err := e.StopRecording(); err == nil {
		t.Fatal("expected not recording")
	}
}

func TestEngineMatchImageAndPreview(t *testing.T) {
	screen, templ := tinyScene()
	dir := t.TempDir()
	path := filepath.Join(dir, "t.png")
	writePNG(t, path, templ)
	e := New(&capClicker{img: screen}, nil)
	raw := e.MatchImage(path, 0.9)
	var hit struct {
		Found bool    `json:"found"`
		X     int     `json:"x"`
		Y     int     `json:"y"`
		Score float64 `json:"score"`
		Error string  `json:"error"`
	}
	if err := json.Unmarshal([]byte(raw), &hit); err != nil {
		t.Fatal(err, raw)
	}
	if !hit.Found || hit.Error != "" || hit.X != 4 || hit.Y != 3 || hit.Score < 0.99 {
		t.Fatalf("%+v %s", hit, raw)
	}
	miss := e.MatchImage(filepath.Join(dir, "missing.png"), 0.9)
	if !strings.Contains(miss, `"found":false`) || !strings.Contains(miss, `"error"`) {
		t.Fatal(miss)
	}

	preview, err := e.RandomPointPreview(20, 20, 5, 30)
	if err != nil {
		t.Fatal(err)
	}
	var pts [][2]int
	if err := json.Unmarshal([]byte(preview), &pts); err != nil {
		t.Fatal(err)
	}
	if len(pts) != 30 {
		t.Fatal(len(pts))
	}
	for _, p := range pts {
		dx, dy := float64(p[0]-20), float64(p[1]-20)
		if math.Hypot(dx, dy) > 5.75 {
			t.Fatalf("outside %v", p)
		}
	}
	if _, err := e.RandomPointPreview(-1, 0, 1, 1); err == nil {
		t.Fatal("expected error")
	}
}

type fakeKey struct {
	mu sync.Mutex
	fn func()
}

func (f *fakeKey) Start(fn func()) error {
	f.mu.Lock()
	f.fn = fn
	f.mu.Unlock()
	return nil
}

func (f *fakeKey) Stop() {
	f.mu.Lock()
	f.fn = nil
	f.mu.Unlock()
}

func (f *fakeKey) trigger() {
	f.mu.Lock()
	fn := f.fn
	f.mu.Unlock()
	if fn != nil {
		fn()
	}
}

type fakeInput struct {
	mu sync.Mutex
	fn func(record.Event)
}

func (f *fakeInput) Start(fn func(record.Event)) error {
	f.mu.Lock()
	f.fn = fn
	f.mu.Unlock()
	return nil
}

func (f *fakeInput) Stop() {
	f.mu.Lock()
	f.fn = nil
	f.mu.Unlock()
}

func (f *fakeInput) emit(ev record.Event) {
	f.mu.Lock()
	fn := f.fn
	f.mu.Unlock()
	if fn != nil {
		fn(ev)
	}
}

func waitKind(t *testing.T, e *Engine, kind string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s, ok := e.PollEvent()
		if ok && strings.Contains(s, `"`+kind+`"`) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("missing %s", kind)
}

func hasKind(kinds []string, want string) bool {
	for _, k := range kinds {
		if k == want {
			return true
		}
	}
	return false
}

func tinyScene() (screen, templ *image.RGBA) {
	templ = image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			v := uint8(0)
			if x == 1 || y == 1 {
				v = 255
			}
			templ.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	screen = image.NewRGBA(image.Rect(0, 0, 12, 10))
	for i := 0; i < len(screen.Pix); i += 4 {
		screen.Pix[i], screen.Pix[i+1], screen.Pix[i+2], screen.Pix[i+3] = 40, 40, 40, 255
	}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			screen.SetRGBA(2+x, 1+y, templ.RGBAAt(x, y))
		}
	}
	return screen, templ
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}
