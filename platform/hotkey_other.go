//go:build !linux && !windows && !darwin

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewHotkey parses name and reports that this OS has no global hotkey.
// Android uses mobile.EmergencyStop from the overlay button instead.
func NewHotkey(name string) (core.HotkeySource, error) {
	if _, err := core.ParseHotkey(name); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("global hotkey is not supported on this OS: %w", ErrUnsupported)
}
