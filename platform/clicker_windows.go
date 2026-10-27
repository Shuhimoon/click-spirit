//go:build windows

package platform

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/click-spirit/click-spirit/core"
)

const (
	inputMouse = 0

	mouseeventfMove        = 0x0001
	mouseeventfLeftdown    = 0x0002
	mouseeventfLeftup      = 0x0004
	mouseeventfRightdown   = 0x0008
	mouseeventfRightup     = 0x0010
	mouseeventfMiddledown  = 0x0020
	mouseeventfMiddleup    = 0x0040
	mouseeventfVirtualdesk = 0x4000
	mouseeventfAbsolute    = 0x8000

	smXVirtualScreen  = 76
	smYVirtualScreen  = 77
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
)

var (
	user32                            = windows.NewLazySystemDLL("user32.dll")
	procSendInput                     = user32.NewProc("SendInput")
	procGetSystemMetrics              = user32.NewProc("GetSystemMetrics")
	procGetCursorPos                  = user32.NewProc("GetCursorPos")
	procSetProcessDPIAware            = user32.NewProc("SetProcessDPIAware")
	procSetProcessDpiAwarenessContext = user32.NewProc("SetProcessDpiAwarenessContext")

	dpiOnce sync.Once
)

// point matches Win32 POINT (two LONG fields).
type point struct {
	X int32
	Y int32
}

type winClicker struct {
	mu   sync.Mutex
	x, y int
}

var (
	_ core.Clicker       = (*winClicker)(nil)
	_ core.CursorLocator = (*winClicker)(nil)
)

// New returns a SendInput-based clicker. The process is marked per-monitor
// DPI aware when the OS allows it, so coordinates match physical pixels.
func New() (core.Clicker, error) {
	checkInputSize()
	dpiOnce.Do(trySetDPIAware)
	return &winClicker{}, nil
}

// Name reports the platform backend.
func Name() string { return "windows" }

// AccessibilityTrusted is always true on Windows. The meaningful check is macOS.
func AccessibilityTrusted() bool { return true }

func checkInputSize() {
	sz := unsafe.Sizeof(input{})
	switch unsafe.Sizeof(uintptr(0)) {
	case 8:
		if sz != 40 {
			panic(fmt.Sprintf("INPUT size %d, want 40", sz))
		}
	case 4:
		if sz != 28 {
			panic(fmt.Sprintf("INPUT size %d, want 28", sz))
		}
	}
}

func trySetDPIAware() {
	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is (HANDLE)-4.
	if procSetProcessDpiAwarenessContext.Find() == nil {
		r, _, _ := procSetProcessDpiAwarenessContext.Call(^uintptr(3))
		if r != 0 {
			return
		}
	}
	if procSetProcessDPIAware.Find() == nil {
		procSetProcessDPIAware.Call()
	}
}

func (w *winClicker) MoveTo(x, y int) error {
	w.mu.Lock()
	w.x, w.y = x, y
	w.mu.Unlock()
	return sendAbsolute(x, y, mouseeventfMove)
}

func (w *winClicker) Click(b core.Button, count, holdMs int) error {
	if count <= 0 {
		count = 1
	}
	down, up, err := buttonFlags(b)
	if err != nil {
		return err
	}
	w.mu.Lock()
	x, y := w.x, w.y
	w.mu.Unlock()
	for i := 0; i < count; i++ {
		if err := sendAbsolute(x, y, mouseeventfMove|down); err != nil {
			return err
		}
		if holdMs > 0 {
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
		}
		if err := sendAbsolute(x, y, mouseeventfMove|up); err != nil {
			return err
		}
	}
	return nil
}

func (w *winClicker) CursorPos() (int, int, error) {
	var pt point
	r, _, err := procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if r == 0 {
		if err != nil {
			return 0, 0, err
		}
		return 0, 0, errors.New("GetCursorPos failed")
	}
	return int(pt.X), int(pt.Y), nil
}

func buttonFlags(b core.Button) (down, up uint32, err error) {
	switch b {
	case core.ButtonLeft:
		return mouseeventfLeftdown, mouseeventfLeftup, nil
	case core.ButtonRight:
		return mouseeventfRightdown, mouseeventfRightup, nil
	case core.ButtonMiddle:
		return mouseeventfMiddledown, mouseeventfMiddleup, nil
	default:
		return 0, 0, fmt.Errorf("invalid button %q", b)
	}
}

func sendAbsolute(x, y int, flags uint32) error {
	in := input{
		typ: inputMouse,
		mi: mouseInput{
			dx:      normAxis(x, systemMetric(smXVirtualScreen), systemMetric(smCXVirtualScreen)),
			dy:      normAxis(y, systemMetric(smYVirtualScreen), systemMetric(smCYVirtualScreen)),
			dwFlags: flags | mouseeventfAbsolute | mouseeventfVirtualdesk,
		},
	}
	return sendInput(&in)
}

// normAxis maps a virtual-desktop pixel into the 0..65535 absolute range.
// The last pixel (origin+span-1) maps to 65535.
func normAxis(value, origin, span int) int32 {
	if span <= 1 {
		return 0
	}
	n := int64(value-origin) * 65535 / int64(span-1)
	if n < 0 {
		n = 0
	}
	if n > 65535 {
		n = 65535
	}
	return int32(n)
}

func systemMetric(idx uintptr) int {
	r, _, _ := procGetSystemMetrics.Call(idx)
	return int(int32(r))
}

func sendInput(in *input) error {
	r, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(in)), unsafe.Sizeof(*in))
	if r != 1 {
		if err != nil {
			return fmt.Errorf("SendInput: %w", err)
		}
		return errors.New("SendInput failed")
	}
	return nil
}
