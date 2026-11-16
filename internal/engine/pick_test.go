package engine

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

type fakePicker struct {
	res  core.PickResult
	err  error
	gate chan struct{}
}

func (f *fakePicker) Pick(mode core.PickMode) (core.PickResult, error) {
	if f.gate != nil {
		<-f.gate
	}
	r := f.res
	r.Mode = mode
	return r, f.err
}

func waitPick(t *testing.T, e *Engine) pickPayload {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if raw, ok := e.PollEvent(); ok {
			var p pickPayload
			if err := json.Unmarshal([]byte(raw), &p); err != nil {
				t.Fatal(err)
			}
			return p
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no pick event")
	return pickPayload{}
}

func TestStartPickQueuesResult(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	if err := e.StartPick(core.PickPoint); !errors.Is(err, ErrPickerUnsupported) {
		t.Fatalf("no picker: err = %v", err)
	}
	e.SetPicker(&fakePicker{res: core.PickResult{X: -1820, Y: 50}})
	if !e.HasPicker() {
		t.Fatal("HasPicker false")
	}
	if err := e.StartPick(core.PickPoint); err != nil {
		t.Fatal(err)
	}
	p := waitPick(t, e)
	if p.Kind != EventPick || p.Mode != "point" || p.X != -1820 || p.Y != 50 || p.Message != "座標 -1820, 50" {
		t.Fatalf("point payload %+v", p)
	}

	e.SetPicker(&fakePicker{res: core.PickResult{X: 200, Y: 200, R: 50}})
	if err := e.StartPick(core.PickCircle); err != nil {
		t.Fatal(err)
	}
	p = waitPick(t, e)
	if p.Kind != EventPick || p.Mode != "circle" || p.X != 200 || p.R != 50 || p.Message != "圓心 200, 200 · r=50" {
		t.Fatalf("circle payload %+v", p)
	}
}

func TestStartPickCancelErrorAndBusy(t *testing.T) {
	e := New(&fakeClicker{}, nil)
	gate := make(chan struct{})
	e.SetPicker(&fakePicker{err: core.ErrPickCanceled, gate: gate})
	if err := e.StartPick(core.PickPoint); err != nil {
		t.Fatal(err)
	}
	if err := e.StartPick(core.PickPoint); !errors.Is(err, ErrPickBusy) {
		t.Fatalf("second pick err = %v", err)
	}
	close(gate)
	if p := waitPick(t, e); p.Kind != EventPickCanceled {
		t.Fatalf("payload %+v", p)
	}
	e.SetPicker(&fakePicker{err: errors.New("no display")})
	if err := e.StartPick(core.PickCircle); err != nil {
		t.Fatal(err)
	}
	if p := waitPick(t, e); p.Kind != EventPickError || p.Message != "no display" {
		t.Fatalf("payload %+v", p)
	}
	if err := e.StartPick("square"); err == nil {
		t.Fatal("bad mode accepted")
	}
}
