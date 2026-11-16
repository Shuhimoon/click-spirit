//go:build linux && cgo && !nox11

package platform

import (
	"image"
	"os"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

func TestX11CountdownOverlay(t *testing.T) {
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
	cd, ok := ov.(core.CountdownOverlay)
	if !ok {
		t.Fatal("x11 overlay does not implement CountdownOverlay")
	}
	cd.ShowCountdown(3)
	time.Sleep(150 * time.Millisecond)
	rects, mapped := xo.state()
	if !mapped || rects != 0 {
		t.Fatalf("countdown card mapped=%v inputRects=%d; want mapped, click-through", mapped, rects)
	}
	clicker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	img, err := clicker.(core.ScreenCapturer).Capture()
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	c := img.(*image.RGBA).RGBAAt(b.Dx()/2, b.Dy()/6+1) // blue card border
	if !(c.B > 200 && c.R < 60) {
		t.Fatalf("no countdown card border at the top of the screen: %+v", c)
	}
	cd.ShowCountdown(0)
	time.Sleep(100 * time.Millisecond)
	if _, mapped := xo.state(); mapped {
		t.Fatal("countdown card still mapped after ShowCountdown(0)")
	}
	cd.ShowCountdown(1)
	time.Sleep(countdownHold + 200*time.Millisecond)
	if _, mapped := xo.state(); mapped {
		t.Fatal("countdown card did not time out")
	}
}
