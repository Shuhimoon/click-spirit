//go:build !windows && !(darwin && cgo)

package platform

import "github.com/click-spirit/click-spirit/core"

// New returns ErrUnsupported. Desktop clicking is implemented on Windows and
// on darwin with cgo. Android callers should use the mobile package.
func New() (core.Clicker, error) {
	return nil, ErrUnsupported
}

// Name reports the platform backend.
func Name() string { return "unsupported" }

// AccessibilityTrusted is true when the OS does not gate clicking on it.
func AccessibilityTrusted() bool { return true }
