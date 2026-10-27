package platform

import (
	"errors"
	"testing"
)

// TestNewHotkeyRejectsBadName must not open a display or register a key.
// A successful NewHotkey("F12") belongs only in the xvfb test.
func TestNewHotkeyRejectsBadName(t *testing.T) {
	src, err := NewHotkey("not-a-key")
	if err == nil || src != nil {
		t.Fatalf("src=%v err=%v", src, err)
	}
}

func TestNewInputSourceUnsupported(t *testing.T) {
	src, err := NewInputSource()
	if src != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("src=%v err=%v", src, err)
	}
}
