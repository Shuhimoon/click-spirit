package platform

import (
	"errors"
	"testing"
)

func TestCaptureNotImplemented(t *testing.T) {
	img, err := TODOCapturer{}.Capture()
	if img != nil || !errors.Is(err, ErrCaptureNotImplemented) {
		t.Fatalf("img=%v err=%v", img, err)
	}
}
