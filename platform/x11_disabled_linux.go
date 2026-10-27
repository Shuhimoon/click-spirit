//go:build linux && (!cgo || nox11)

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

func x11BuiltIn() bool { return false }

// newX11 reports that this build has no X11 backend. Selection code still
// compiles when cgo is off or the binary is built with -tags nox11.
func newX11() (core.Clicker, error) {
	return nil, fmt.Errorf("X11 backend is not included in this build (cgo disabled or -tags nox11): %w", ErrUnsupported)
}
