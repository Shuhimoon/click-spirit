//go:build linux && cgo && !nox11

package platform

import (
	"image"
	"os"
	"testing"

	"github.com/click-spirit/click-spirit/core"
)

func TestRGBAFromZPixmap(t *testing.T) {
	// Little-endian 32-bit pixel: memory B, G, R, pad with masks ff0000/ff00/ff.
	raw := []byte{0x10, 0x20, 0x30, 0x00}
	img, err := rgbaFromZPixmap(1, 1, 32, 4, 0, 0x00ff0000, 0x0000ff00, 0x000000ff, raw)
	if err != nil {
		t.Fatal(err)
	}
	c := img.RGBAAt(0, 0)
	if c.R != 0x30 || c.G != 0x20 || c.B != 0x10 || c.A != 0xff {
		t.Fatalf("%+v", c)
	}

	// MSBFirst stores the same uint32 as 00 30 20 10.
	rawMSB := []byte{0x00, 0x30, 0x20, 0x10}
	img, err = rgbaFromZPixmap(1, 1, 32, 4, 1, 0x00ff0000, 0x0000ff00, 0x000000ff, rawMSB)
	if err != nil {
		t.Fatal(err)
	}
	c = img.RGBAAt(0, 0)
	if c.R != 0x30 || c.G != 0x20 || c.B != 0x10 || c.A != 0xff {
		t.Fatalf("msb %+v", c)
	}
}

// TestX11MoveAndCapture runs only when CLICK_SPIRIT_X11_TEST=1.
// Invoke it under a private server:
//
//	xvfb-run -a -s "-screen 0 1280x800x24" env CLICK_SPIRIT_X11_TEST=1 go test ./platform -run X11 -v
//
// It refuses the shared displays other sessions use (:7, :14, :15, :17).
func TestX11MoveAndCapture(t *testing.T) {
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
	t.Setenv("CLICK_SPIRIT_BACKEND", "x11")

	clicker, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if c, ok := clicker.(interface{ Close() error }); ok {
		defer c.Close()
	}
	if err := clicker.MoveTo(10, 20); err != nil {
		t.Fatal(err)
	}
	loc, ok := clicker.(core.CursorLocator)
	if !ok {
		t.Fatal("x11 clicker is not a CursorLocator")
	}
	x, y, err := loc.CursorPos()
	if err != nil {
		t.Fatal(err)
	}
	if x != 10 || y != 20 {
		t.Fatalf("cursor %d,%d want 10,20", x, y)
	}
	cap, ok := clicker.(core.ScreenCapturer)
	if !ok {
		t.Fatal("x11 clicker is not a ScreenCapturer")
	}
	img, err := cap.Capture()
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Empty() || b.Dx() <= 0 || b.Dy() <= 0 {
		t.Fatalf("bounds %v", b)
	}
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("image type %T", img)
	}
	if rgba.RGBAAt(0, 0).A != 255 {
		t.Fatal("expected opaque pixel")
	}
}
