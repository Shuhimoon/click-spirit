package engine

import (
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

type fakeBubble struct {
	mu        sync.Mutex
	rect      core.Rect
	visible   bool
	recording bool
	running   bool
	fn        func(string)
}

func (b *fakeBubble) SetVisible(on bool) { b.mu.Lock(); b.visible = on; b.mu.Unlock() }
func (b *fakeBubble) Rect() (core.Rect, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.rect, b.visible
}
func (b *fakeBubble) SetState(rec, run bool) {
	b.mu.Lock()
	b.recording, b.running = rec, run
	b.mu.Unlock()
}
func (b *fakeBubble) SetOnAction(fn func(string)) { b.fn = fn }

type fakeVideo struct {
	path    string
	stopped bool
	err     error
}

func (v *fakeVideo) Start(path string) error { v.path = path; return v.err }
func (v *fakeVideo) Stop() error             { v.stopped = true; return nil }

func drain(t *testing.T, e *Engine) []eventPayload {
	t.Helper()
	var out []eventPayload
	for {
		raw, ok := e.PollEvent()
		if !ok {
			return out
		}
		var p eventPayload
		if err := json.Unmarshal([]byte(raw), &p); err != nil {
			t.Fatal(err)
		}
		out = append(out, p)
	}
}

func TestBubbleActionsExcludeAndRecordingFilter(t *testing.T) {
	cl := &fakeClicker{}
	e := New(cl, nil)
	e.Sleep = instant
	b := &fakeBubble{rect: core.Rect{X: 1500, Y: 900, W: 60, H: 60}}
	e.SetBubble(b)
	if !e.HasBubble() {
		t.Fatal("HasBubble false")
	}
	e.ShowBubble(true)

	b.fn(core.BubbleStart)
	evs := drain(t, e)
	if len(evs) != 1 || evs[0].Kind != EventBubble || evs[0].Message != "start" {
		t.Fatalf("bubble events %+v", evs)
	}

	// Scripts never click inside the bubble.
	raw := scriptJSON(t, &core.Script{StartDelaySec: core.StartDelay(0), Version: 1, Repeat: 1, Steps: []core.Step{
		{X: 1510, Y: 910, Button: core.ButtonLeft},
		{X: 10, Y: 20, Button: core.ButtonLeft},
	}})
	if err := e.Start(raw); err != nil {
		t.Fatal(err)
	}
	if err := e.Wait(); err != nil {
		t.Fatal(err)
	}
	cl.mu.Lock()
	clicks, x, y := cl.clicks, cl.x, cl.y
	cl.mu.Unlock()
	if clicks != 1 || x != 10 || y != 20 {
		t.Fatalf("clicks=%d last=(%d,%d)", clicks, x, y)
	}
	b.mu.Lock()
	stillRunning := b.running
	b.mu.Unlock()
	if stillRunning {
		t.Fatal("bubble still shows a running script after it finished")
	}
	excluded := false
	for _, p := range drain(t, e) {
		if p.Kind == string(core.EventExcluded) {
			excluded = true
		}
	}
	if !excluded {
		t.Fatal("no excluded event")
	}

	// Presses on the bubble are not recorded.
	src := &fakeInput{}
	e.SetInputFactory(func() (record.InputSource, error) { return src, nil })
	if err := e.StartRecording(); err != nil {
		t.Fatal(err)
	}
	if !b.recording {
		t.Fatal("bubble not told about recording")
	}
	t0 := time.Unix(100, 0)
	src.emit(record.Event{Kind: record.KindDown, Button: core.ButtonLeft, X: 1520, Y: 920, Time: t0})
	src.emit(record.Event{Kind: record.KindUp, Button: core.ButtonLeft, X: 1520, Y: 920, Time: t0.Add(50 * time.Millisecond)})
	src.emit(record.Event{Kind: record.KindDown, Button: core.ButtonLeft, X: 30, Y: 40, Time: t0.Add(time.Second)})
	src.emit(record.Event{Kind: record.KindUp, Button: core.ButtonLeft, X: 30, Y: 40, Time: t0.Add(1100 * time.Millisecond)})
	out, err := e.StopRecording()
	if err != nil {
		t.Fatal(err)
	}
	s, _ := core.Unmarshal([]byte(out))
	if len(s.Steps) != 1 || s.Steps[0].X != 30 {
		t.Fatalf("recorded %+v", s.Steps)
	}
	if b.recording {
		t.Fatal("bubble still shows recording")
	}
}

func TestScreenRecordingAndClicksToScript(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	src := &fakeInput{}
	e.SetInputFactory(func() (record.InputSource, error) { return src, nil })

	// No video backend: clicks only, with a note.
	if err := e.StartScreenRecording("/tmp/x.mp4"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	src.emit(record.Event{Kind: record.KindClick, Button: core.ButtonLeft, X: 5, Y: 6, Time: now.Add(700 * time.Millisecond)})
	var res ScreenRecording
	if err := json.Unmarshal([]byte(e.StopScreenRecording()), &res); err != nil {
		t.Fatal(err)
	}
	if res.Video != "" || res.Note == "" || len(res.Clicks) != 1 || res.Clicks[0].X != 5 {
		t.Fatalf("no-video result %+v", res)
	}
	if res.Clicks[0].AtMs < 600 || res.Clicks[0].AtMs > 800 {
		t.Fatalf("timecode %d", res.Clicks[0].AtMs)
	}

	v := &fakeVideo{}
	e.SetVideoRecorder(v)
	if err := e.StartScreenRecording("/tmp/rec.mp4"); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(e.StopScreenRecording()), &res)
	if res.Video != "/tmp/rec.mp4" || v.path != "/tmp/rec.mp4" || !v.stopped || len(res.Clicks) != 0 {
		t.Fatalf("video result %+v %+v", res, v)
	}

	v.err = errors.New("no ffmpeg")
	_ = e.StartScreenRecording("/tmp/rec2.mp4")
	_ = json.Unmarshal([]byte(e.StopScreenRecording()), &res)
	if res.Video != "" || res.Note == "" {
		t.Fatalf("failed video result %+v", res)
	}

	js, err := ClicksToScript(`[{"x":300,"y":400,"at_ms":2000},{"x":100,"y":200,"at_ms":500,"hold_ms":80}]`, "影片")
	if err != nil {
		t.Fatal(err)
	}
	s, _ := core.Unmarshal([]byte(js))
	if s.Name != "影片" || len(s.Steps) != 2 || s.Steps[0].X != 100 || s.Steps[0].IntervalMs != 1420 || s.Steps[1].IntervalMs != 500 {
		t.Fatalf("script %+v", s)
	}
	if _, err := ClicksToScript("nope", ""); err == nil {
		t.Fatal("bad JSON accepted")
	}
}
