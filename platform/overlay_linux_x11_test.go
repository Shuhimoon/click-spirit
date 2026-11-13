//go:build linux && cgo && !nox11

package platform

import (
	"image"
	"os"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// TestX11ClickOverlay runs only when CLICK_SPIRIT_X11_TEST=1 under xvfb-run:
//
//	xvfb-run -a -s "-screen 0 1280x800x24" env CLICK_SPIRIT_X11_TEST=1 go test ./platform -run Overlay -v
func TestX11ClickOverlay(t *testing.T) {
	if os.Getenv("CLICK_SPIRIT_X11_TEST") != "1" {
		t.Skip("set CLICK_SPIRIT_X11_TEST=1 under xvfb-run")
	}
	disp := os.Getenv("DISPLAY")
	if disp == "" || forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing display %q; run under xvfb-run", disp)
	}
	t.Setenv("CLICK_SPIRIT_BACKEND", "x11")

	ov, err := NewClickOverlay()
	if err != nil {
		t.Fatal(err)
	}
	xo := ov.(*x11Overlay)
	defer xo.Close()

	clicker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	capt := clicker.(core.ScreenCapturer)

	const cx, cy = 300, 200
	ov.ShowClick(cx, cy, "步驟 2 · 第 3/10 次")
	time.Sleep(rippleFrames*rippleFrameDur + 120*time.Millisecond)

	rects, mapped := xo.state()
	if !mapped {
		t.Fatal("overlay window is not mapped after ShowClick")
	}
	if rects != 0 {
		t.Fatalf("overlay input shape has %d rectangles; want 0 (click-through)", rects)
	}

	img, err := capt.Capture()
	if err != nil {
		t.Fatal(err)
	}
	isBlue := func(x, y int) bool {
		c := img.(*image.RGBA).RGBAAt(x, y)
		return c.B > 200 && c.R < 60 && c.G > 100 && c.G < 170
	}
	if !isBlue(cx, cy) {
		t.Fatalf("center dot at (%d,%d) is %+v", cx, cy, img.At(cx, cy))
	}
	if !isBlue(cx+rippleRMax-1, cy) || !isBlue(cx, cy-rippleRMax+1) {
		t.Fatalf("ripple ring not drawn at radius %d", rippleRMax)
	}
	if isBlue(cx+16, cy) {
		t.Fatal("inside of the ring should be see-through")
	}

	// The window hides itself after the hold time.
	time.Sleep(rippleHold + 200*time.Millisecond)
	if _, mapped := xo.state(); mapped {
		t.Fatal("overlay still mapped after the hold time")
	}
}
