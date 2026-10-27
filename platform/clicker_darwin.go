//go:build darwin && cgo

package platform

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>

static int csClickButton(int x, int y, int which, int phase, int clickState) {
	CGEventType type;
	CGMouseButton button = kCGMouseButtonLeft;
	if (phase == 0) {
		type = kCGEventMouseMoved;
		button = kCGMouseButtonLeft;
	} else if (which == 1) {
		button = kCGMouseButtonRight;
		type = (phase == 1) ? kCGEventRightMouseDown : kCGEventRightMouseUp;
	} else if (which == 2) {
		button = kCGMouseButtonCenter;
		type = (phase == 1) ? kCGEventOtherMouseDown : kCGEventOtherMouseUp;
	} else {
		button = kCGMouseButtonLeft;
		type = (phase == 1) ? kCGEventLeftMouseDown : kCGEventLeftMouseUp;
	}
	CGPoint pt;
	pt.x = (CGFloat)x;
	pt.y = (CGFloat)y;
	CGEventRef ev = CGEventCreateMouseEvent(NULL, type, pt, button);
	if (ev == NULL) {
		return -1;
	}
	if (clickState > 0) {
		CGEventSetIntegerValueField(ev, kCGMouseEventClickState, (int64_t)clickState);
	}
	CGEventPost(kCGHIDEventTap, ev);
	CFRelease(ev);
	return 0;
}

static int csCursorPos(int *x, int *y) {
	CGEventRef ev = CGEventCreate(NULL);
	if (ev == NULL) {
		return -1;
	}
	CGPoint pt = CGEventGetLocation(ev);
	CFRelease(ev);
	if (x) *x = (int)pt.x;
	if (y) *y = (int)pt.y;
	return 0;
}

static int csAccessibilityTrusted(void) {
	return AXIsProcessTrusted() ? 1 : 0;
}
*/
import "C"
import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

type darwinClicker struct {
	mu   sync.Mutex
	x, y int
}

var (
	_ core.Clicker       = (*darwinClicker)(nil)
	_ core.CursorLocator = (*darwinClicker)(nil)
)

// New returns a CoreGraphics clicker. The process needs Accessibility permission.
func New() (core.Clicker, error) {
	return &darwinClicker{}, nil
}

// Name reports the platform backend.
func Name() string { return "darwin" }

// AccessibilityTrusted reports whether this process may post HID events.
func AccessibilityTrusted() bool {
	return C.csAccessibilityTrusted() != 0
}

func (d *darwinClicker) MoveTo(x, y int) error {
	d.mu.Lock()
	d.x, d.y = x, y
	d.mu.Unlock()
	if C.csClickButton(C.int(x), C.int(y), 0, 0, 0) != 0 {
		return errors.New("CGEventCreateMouseEvent failed")
	}
	return nil
}

func (d *darwinClicker) Click(b core.Button, count, holdMs int) error {
	if count <= 0 {
		count = 1
	}
	which, err := darwinButton(b)
	if err != nil {
		return err
	}
	d.mu.Lock()
	x, y := d.x, d.y
	d.mu.Unlock()
	for i := 1; i <= count; i++ {
		if C.csClickButton(C.int(x), C.int(y), C.int(which), 1, C.int(i)) != 0 {
			return errors.New("mouse down failed")
		}
		if holdMs > 0 {
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
		}
		if C.csClickButton(C.int(x), C.int(y), C.int(which), 2, C.int(i)) != 0 {
			return errors.New("mouse up failed")
		}
	}
	return nil
}

func (d *darwinClicker) CursorPos() (int, int, error) {
	var x, y C.int
	if C.csCursorPos(&x, &y) != 0 {
		return 0, 0, errors.New("CGEventCreate failed")
	}
	return int(x), int(y), nil
}

func darwinButton(b core.Button) (int, error) {
	switch b {
	case core.ButtonLeft:
		return 0, nil
	case core.ButtonRight:
		return 1, nil
	case core.ButtonMiddle:
		return 2, nil
	default:
		return 0, fmt.Errorf("invalid button %q", b)
	}
}
