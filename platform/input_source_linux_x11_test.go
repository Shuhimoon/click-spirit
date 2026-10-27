//go:build linux && cgo && !nox11

package platform

import (
	"os"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
	"github.com/click-spirit/click-spirit/internal/engine"
)

// TestX11RecordClick runs under xvfb. It skips when DISPLAY is empty or the
// server has no RECORD extension, and it refuses the shared displays.
//
//	xvfb-run -a go test ./platform/ -run X11Record -v
func TestX11RecordClick(t *testing.T) {
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		t.Skip("DISPLAY is empty")
	}
	if forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing to send X11 events to shared display %q", disp)
	}
	t.Setenv("CLICK_SPIRIT_BACKEND", "x11")
	if !RecordingUsesX11() {
		t.Skip("XRecord extension is not available")
	}

	clicker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := clicker.(interface{ Close() error }); ok {
		defer c.Close()
	}

	assertRecordedClick(t, clicker, 80, 90)
	// Stop is idempotent and a fresh source can record again.
	assertRecordedClick(t, clicker, 110, 130)
}

func assertRecordedClick(t *testing.T, clicker core.Clicker, x, y int) {
	t.Helper()
	src, err := NewInputSource()
	if err != nil {
		t.Fatal(err)
	}
	got := make(chan record.Event, 64)
	if err := src.Start(func(ev record.Event) {
		if ev.Kind != record.KindDown && ev.Kind != record.KindUp {
			return
		}
		select {
		case got <- ev:
		default:
		}
	}); err != nil {
		src.Stop()
		t.Fatal(err)
	}
	defer src.Stop()

	time.Sleep(50 * time.Millisecond)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := clicker.MoveTo(x, y); err != nil {
			t.Fatal(err)
		}
		if err := clicker.Click(core.ButtonLeft, 1, 15); err != nil {
			t.Fatal(err)
		}
		var down, up bool
		wait := time.NewTimer(200 * time.Millisecond)
		collecting := true
		for collecting {
			select {
			case ev := <-got:
				if ev.X == x && ev.Y == y && ev.Button == core.ButtonLeft {
					if ev.Kind == record.KindDown {
						down = true
					}
					if ev.Kind == record.KindUp {
						up = true
					}
				}
				if down && up {
					wait.Stop()
					return
				}
			case <-wait.C:
				collecting = false
			}
		}
	}
	t.Fatalf("missing down/up at %d,%d", x, y)
}

// TestEngineX11Recording checks StartRecording/StopRecording against XRecord.
func TestEngineX11Recording(t *testing.T) {
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		t.Skip("DISPLAY is empty")
	}
	if forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing to send X11 events to shared display %q", disp)
	}
	t.Setenv("CLICK_SPIRIT_BACKEND", "x11")
	if !RecordingUsesX11() {
		t.Skip("XRecord extension is not available")
	}

	clicker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := clicker.(interface{ Close() error }); ok {
		defer c.Close()
	}
	e := engine.New(clicker, nil)
	e.SetInputFactory(NewInputSource)
	e.SetRecordOptions(record.Options{DoubleClickMs: -1, MinIntervalMs: 0})
	if err := e.StartRecording(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(80 * time.Millisecond)
	const x, y = 150, 240
	if err := clicker.MoveTo(x, y); err != nil {
		_, _ = e.StopRecording()
		t.Fatal(err)
	}
	if err := clicker.Click(core.ButtonLeft, 1, 20); err != nil {
		_, _ = e.StopRecording()
		t.Fatal(err)
	}
	time.Sleep(500 * time.Millisecond)
	raw, err := e.StopRecording()
	if err != nil {
		t.Fatal(err)
	}
	script, err := core.Unmarshal([]byte(raw))
	if err != nil {
		t.Fatal(err, raw)
	}
	if err := script.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, step := range script.Steps {
		if step.X == x && step.Y == y && step.Button == core.ButtonLeft && step.Clicks >= 1 {
			return
		}
	}
	t.Fatalf("no step at %d,%d in %+v", x, y, script.Steps)
}
