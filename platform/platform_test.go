package platform

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestStubOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	c, err := New()
	if c != nil {
		t.Fatalf("clicker = %#v", c)
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "mobile") || !strings.Contains(strings.ToLower(err.Error()), "android") {
		t.Fatalf("error should point Android users at the mobile package: %v", err)
	}
	if Name() != "unsupported" {
		t.Fatal(Name())
	}
	if !AccessibilityTrusted() {
		t.Fatal("expected trusted on non-darwin stub")
	}
}

func TestCaptureNotImplemented(t *testing.T) {
	img, err := TODOCapturer{}.Capture()
	if img != nil || !errors.Is(err, ErrCaptureNotImplemented) {
		t.Fatalf("img=%v err=%v", img, err)
	}
}
