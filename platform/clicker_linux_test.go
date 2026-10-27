//go:build linux

package platform

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func TestLinuxBackendSelection(t *testing.T) {
	t.Run("force uinput", func(t *testing.T) {
		t.Setenv("CLICK_SPIRIT_BACKEND", "uinput")
		if Name() != "uinput" {
			t.Fatal(Name())
		}
	})
	t.Run("force x11", func(t *testing.T) {
		t.Setenv("CLICK_SPIRIT_BACKEND", "x11")
		if Name() != "x11" {
			t.Fatal(Name())
		}
	})
	t.Run("unknown", func(t *testing.T) {
		t.Setenv("CLICK_SPIRIT_BACKEND", "nope")
		if Name() != "unsupported" {
			t.Fatal(Name())
		}
		// Unknown backend fails before any device is opened.
		c, err := New()
		if c != nil || err == nil {
			t.Fatal(c, err)
		}
	})
	t.Run("display prefers x11 when built", func(t *testing.T) {
		t.Setenv("CLICK_SPIRIT_BACKEND", "")
		t.Setenv("DISPLAY", ":99")
		t.Setenv("WAYLAND_DISPLAY", "wayland-0")
		if x11BuiltIn() {
			if Name() != "x11" {
				t.Fatal(Name())
			}
		} else if Name() != "uinput" {
			t.Fatal(Name())
		}
	})
	t.Run("wayland without display", func(t *testing.T) {
		t.Setenv("CLICK_SPIRIT_BACKEND", "")
		t.Setenv("DISPLAY", "")
		t.Setenv("WAYLAND_DISPLAY", "wayland-0")
		if Name() != "uinput" {
			t.Fatal(Name())
		}
	})
	t.Run("no display", func(t *testing.T) {
		t.Setenv("CLICK_SPIRIT_BACKEND", "")
		t.Setenv("DISPLAY", "")
		t.Setenv("WAYLAND_DISPLAY", "")
		if x11BuiltIn() {
			if Name() != "unsupported" {
				t.Fatal(Name())
			}
			c, err := New()
			if c != nil || !errors.Is(err, ErrUnsupported) {
				t.Fatal(c, err)
			}
			return
		}
		if Name() != "uinput" {
			t.Fatal(Name())
		}
	})
}

func TestForbiddenSharedDisplay(t *testing.T) {
	if !forbiddenSharedDisplay(":17") || !forbiddenSharedDisplay("localhost:7.0") {
		t.Fatal("expected forbidden")
	}
	if forbiddenSharedDisplay(":99") || forbiddenSharedDisplay("") {
		t.Fatal("expected allowed")
	}
}

// forbiddenSharedDisplay refuses displays that other people are using.
// The X11 integration test must run under xvfb-run, never on these.
func forbiddenSharedDisplay(disp string) bool {
	n, ok := displayNumber(disp)
	if !ok {
		return false
	}
	switch n {
	case 7, 14, 15, 17:
		return true
	default:
		return false
	}
}

func displayNumber(disp string) (int, bool) {
	i := strings.LastIndex(disp, ":")
	if i < 0 || i+1 >= len(disp) {
		return 0, false
	}
	rest := disp[i+1:]
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		rest = rest[:dot]
	}
	n, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false
	}
	return n, true
}
