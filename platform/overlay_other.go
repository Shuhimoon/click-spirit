//go:build !linux || !cgo || nox11

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewClickOverlay is not implemented on this platform or build yet. The app
// still shows click positions inside its own window.
func NewClickOverlay() (core.ClickOverlay, error) {
	return nil, fmt.Errorf("click overlay is not available on this platform: %w", ErrUnsupported)
}
