//go:build !linux || !cgo || nox11

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewScreenPicker is not implemented on this platform or build yet. The app
// falls back to its in-window capture screen where screen capture exists.
func NewScreenPicker() (core.ScreenPicker, error) {
	return nil, fmt.Errorf("screen picker is not available on this platform: %w", ErrUnsupported)
}
