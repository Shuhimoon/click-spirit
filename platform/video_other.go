//go:build !linux

package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core"
)

// NewVideoRecorder is not implemented on this platform yet; screen
// recordings capture click positions only.
func NewVideoRecorder() (core.VideoRecorder, error) {
	return nil, fmt.Errorf("screen video is not available on this platform: %w", ErrUnsupported)
}

// VideoFrame is not implemented on this platform yet.
func VideoFrame(video string, atMs int64, out string, width int) error {
	return fmt.Errorf("video thumbnails are not available on this platform: %w", ErrUnsupported)
}
