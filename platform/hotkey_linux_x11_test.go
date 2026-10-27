//go:build linux && cgo && !nox11

package platform

import (
	"os"
	"testing"
	"time"
)

// TestX11HotkeyF12 runs only when CLICK_SPIRIT_X11_TEST=1, under xvfb-run.
// It refuses shared displays (:7, :14, :15, :17) before any XTest call.
//
//	xvfb-run -a -s "-screen 0 1280x800x24" env CLICK_SPIRIT_X11_TEST=1 go test ./platform -run X11Hotkey -v
func TestX11HotkeyF12(t *testing.T) {
	if os.Getenv("CLICK_SPIRIT_X11_TEST") != "1" {
		t.Skip("set CLICK_SPIRIT_X11_TEST=1 under xvfb-run")
	}
	disp := os.Getenv("DISPLAY")
	if disp == "" {
		t.Fatal("CLICK_SPIRIT_X11_TEST=1 but DISPLAY is empty")
	}
	if forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing to send X11 events to shared display %q; run under xvfb-run", disp)
	}

	src, err := NewHotkey("F12")
	if err != nil {
		t.Fatal(err)
	}
	fired := make(chan struct{}, 1)
	if err := src.Start(func() {
		select {
		case fired <- struct{}{}:
		default:
		}
	}); err != nil {
		src.Stop()
		t.Fatal(err)
	}
	defer src.Stop()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := x11TapKey("F12"); err != nil {
			t.Fatal(err)
		}
		select {
		case <-fired:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
	t.Fatal("F12 was not delivered to the XGrabKey loop")
}
