//go:build linux

package platform

import (
	"errors"

	"golang.org/x/sys/unix"

	"github.com/click-spirit/click-spirit/core"
)

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

// NewHotkey parses name and returns the Linux chord hook.
// X11 XGrabKey is used when a display can be opened. Otherwise the hook
// reads evdev keyboards (/dev/input/event*), which needs the input group.
func NewHotkey(name string) (core.HotkeySource, error) {
	chord, err := core.ParseHotkey(name)
	if err != nil {
		return nil, err
	}
	if x11HotkeyAvailable() {
		return newX11Hotkey(chord)
	}
	return newEvdevHotkey(chord)
}

// pollX waits until the X connection or the wake pipe is readable.
// A wake, a hangup, or a closed wake fd ends the loop.
func pollX(xfd, wake int) error {
	fds := []unix.PollFd{
		{Fd: int32(xfd), Events: unix.POLLIN},
		{Fd: int32(wake), Events: unix.POLLIN},
	}
	for {
		n, err := unix.Poll(fds, 200)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if fds[1].Revents != 0 {
			return errors.New("hotkey wake")
		}
		if fds[0].Revents != 0 || n == 0 {
			return nil
		}
	}
}
