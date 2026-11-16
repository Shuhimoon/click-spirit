//go:build !linux || !cgo || nox11

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewBubble is not implemented on this platform yet (Windows/macOS stub).
func NewBubble() (core.Bubble, error) {
	return nil, fmt.Errorf("floating bubble is not available on this platform: %w", ErrUnsupported)
}
