//go:build linux && (!cgo || nox11)

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

func x11BuiltIn() bool { return false }

func x11HotkeyAvailable() bool { return false }

func x11RecordAvailable() bool { return false }

func x11PointerInfo() (x, y, w, h int, ok bool) { return 0, 0, 0, 0, false }

// newX11 reports that this build has no X11 backend. Selection code still
// compiles when cgo is off or the binary is built with -tags nox11.
func newX11() (core.Clicker, error) {
	return nil, fmt.Errorf("X11 backend is not included in this build (cgo disabled or -tags nox11): %w", ErrUnsupported)
}

func newX11Hotkey(core.Chord) (core.HotkeySource, error) {
	return nil, fmt.Errorf("X11 hotkey is not included in this build (cgo disabled or -tags nox11): %w", ErrUnsupported)
}

func newX11InputSource() (record.InputSource, error) {
	return nil, fmt.Errorf("X11 recording is not included in this build (cgo disabled or -tags nox11): %w", ErrUnsupported)
}
