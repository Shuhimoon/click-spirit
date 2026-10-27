package platform

import (
	"errors"
	"image"

	"github.com/click-spirit/click-spirit/core"
)

// ErrCaptureNotImplemented is returned by the placeholder capturer.
// TODO: implement Capture for Windows, macOS, and Android image-match clicking.
var ErrCaptureNotImplemented = errors.New("screen capture not implemented")

// TODOCapturer is a placeholder core.ScreenCapturer.
type TODOCapturer struct{}

// Capture is not implemented.
func (TODOCapturer) Capture() (image.Image, error) {
	return nil, ErrCaptureNotImplemented
}

var _ core.ScreenCapturer = TODOCapturer{}
