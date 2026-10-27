//go:build linux

package platform

import "github.com/click-spirit/click-spirit/core"

// X11 modifier masks. Lock and Mod2 are added at grab time so Caps Lock and
// Num Lock do not hide the chord.
const (
	x11Shift   = 1
	x11Lock    = 2
	x11Control = 4
	x11Mod1    = 8  // Alt
	x11Mod2    = 16 // Num Lock
	x11Mod4    = 64 // Super
)

// x11Mods is the base mask for chord, without Lock or Num Lock.
func x11Mods(c core.Chord) uint {
	var m uint
	if c.Shift {
		m |= x11Shift
	}
	if c.Ctrl {
		m |= x11Control
	}
	if c.Alt {
		m |= x11Mod1
	}
	if c.Super {
		m |= x11Mod4
	}
	return m
}
