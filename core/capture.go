package core

import "image"

// ScreenCapturer captures the screen for a future image-match clicker.
// Platform packages do not implement it yet.
type ScreenCapturer interface {
	Capture() (image.Image, error)
}
