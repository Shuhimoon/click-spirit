package core

import "image"

// ScreenCapturer captures the screen for a future image-match clicker.
// The Linux X11 backend implements it. Windows, macOS, Android, and the
// Linux uinput backend do not.
type ScreenCapturer interface {
	Capture() (image.Image, error)
}
