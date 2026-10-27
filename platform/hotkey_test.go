package platform

import (
	"errors"
	"runtime"
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
	if runtime.GOOS == "linux" {
		t.Skip("linux selects XRecord or evdev")
	}
	src, err := NewInputSource()
	if src != nil || !errors.Is(err, ErrUnsupported) {
		t.Fatalf("src=%v err=%v", src, err)
	}
}
