//go:build linux

package platform

import (
	"fmt"
	"os"
	"strings"

	"github.com/click-spirit/click-spirit/core"
)

// Linux backend selection:
//
//	CLICK_SPIRIT_BACKEND=x11|uinput  forces that backend.
//	Otherwise, if DISPLAY is set and the X11 backend is compiled in, use X11.
//	Otherwise, if WAYLAND_DISPLAY is set or X11 was not compiled in, use uinput.
//	Otherwise New returns an error.
//
// uinput is the Wayland path. It cannot read the pointer or capture the
// screen. Its coordinate range comes from CLICK_SPIRIT_SCREEN=WxH or from
// NewUinput. See clicker_linux_uinput.go.

func chooseBackend() (string, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("CLICK_SPIRIT_BACKEND"))) {
	case "x11":
		return "x11", nil
	case "uinput":
		return "uinput", nil
	case "":
	default:
		return "", fmt.Errorf("unknown CLICK_SPIRIT_BACKEND %q (want x11 or uinput)", os.Getenv("CLICK_SPIRIT_BACKEND"))
	}
	if os.Getenv("DISPLAY") != "" && x11BuiltIn() {
		return "x11", nil
	}
	if os.Getenv("WAYLAND_DISPLAY") != "" || !x11BuiltIn() {
		return "uinput", nil
	}
	return "", fmt.Errorf("%w: no DISPLAY or WAYLAND_DISPLAY; set CLICK_SPIRIT_BACKEND=x11|uinput", ErrUnsupported)
}

// New returns the Linux clicker selected by chooseBackend.
func New() (core.Clicker, error) {
	name, err := chooseBackend()
	if err != nil {
		return nil, err
	}
	switch name {
	case "x11":
		return newX11()
	case "uinput":
		return newUinput()
	default:
		return nil, fmt.Errorf("unknown backend %q", name)
	}
}

// Name reports the backend chooseBackend would use: "x11", "uinput", or
// "unsupported" when selection fails. It does not open a display or uinput.
func Name() string {
	name, err := chooseBackend()
	if err != nil {
		return "unsupported"
	}
	return name
}

// AccessibilityTrusted is always true on Linux. X11 does not use the macOS
// accessibility gate. uinput is limited by /dev/uinput permissions instead.
func AccessibilityTrusted() bool { return true }
