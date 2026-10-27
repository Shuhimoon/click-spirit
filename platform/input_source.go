package platform

import (
	"fmt"

	"github.com/click-spirit/click-spirit/core/record"
)

// NewInputSource is the platform pointer hook used by engine.StartRecording.
// Global hooks are not implemented yet. Callers can still record with
// record.Recorder.Add or engine.StartRecordingWith and a test double.
func NewInputSource() (record.InputSource, error) {
	return nil, fmt.Errorf("input hooks are not implemented yet: %w", ErrUnsupported)
}
