//go:build linux && (!cgo || nox11)

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewHotkey parses name and reports that this build has no X11 grab.
// Selection still compiles when cgo is off or the binary uses -tags nox11.
func NewHotkey(name string) (core.HotkeySource, error) {
	if _, err := core.ParseHotkey(name); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("X11 hotkey is not included in this build (cgo disabled or -tags nox11): %w", ErrUnsupported)
}
