//go:build darwin

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewHotkey parses name and reports that the macOS hook is not implemented.
// TODO: a global hotkey needs CGEventTap plus the Accessibility permission,
// on a run-loop thread. Until that exists, call Engine.EmergencyStop from the UI.
func NewHotkey(name string) (core.HotkeySource, error) {
	if _, err := core.ParseHotkey(name); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("macOS emergency hotkey is not implemented (needs CGEventTap and Accessibility): %w", ErrUnsupported)
}
