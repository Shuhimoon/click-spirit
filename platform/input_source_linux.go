//go:build linux

package platform

import (
	"os"

	"github.com/click-spirit/click-spirit/core/record"
)

// NewInputSource returns the Linux pointer hook used by engine.StartRecording.
//
// XRecord is used when the clicker backend is X11 (see chooseBackend: DISPLAY
// is set, the binary includes X11, and CLICK_SPIRIT_BACKEND is not uinput)
// and the server reports the RECORD extension. Otherwise events come from
// evdev (/dev/input/event*). Wayland coordinates from evdev are approximate.
func NewInputSource() (record.InputSource, error) {
	if x11RecordAvailable() {
		return newX11InputSource()
	}
	return newEvdevInputSource()
}

// RecordingUsesX11 reports whether NewInputSource will record with XRecord.
// It is false when the X11 backend is not selected or RECORD is missing.
func RecordingUsesX11() bool { return x11RecordAvailable() }

// pointerSeed is the evdev cursor origin and the clamp rectangle.
// The origin is the X11 backend's pointer when that backend is active and
// can be queried, otherwise 0,0. The rectangle is CLICK_SPIRIT_SCREEN when
// that variable parses, otherwise the X11 screen size when the X11 backend
// is active. Both stay zero when they are unknown.
func pointerSeed() (x, y, w, h int) {
	if s := os.Getenv("CLICK_SPIRIT_SCREEN"); s != "" {
		if sw, sh, err := parseScreenSize(s); err == nil {
			w, h = sw, sh
		}
	}
	name, err := chooseBackend()
	if err == nil && name == "x11" {
		if px, py, sw, sh, ok := x11PointerInfo(); ok {
			x, y = px, py
			if w <= 0 {
				w = sw
			}
			if h <= 0 {
				h = sh
			}
		}
	}
	x, y = clampToScreen(x, y, w, h)
	return x, y, w, h
}
