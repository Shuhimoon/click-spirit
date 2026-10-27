//go:build linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/click-spirit/click-spirit/core"
)

// uinput is the Wayland (and headless) backend. Wayland compositors do not
// expose the pointer position or the framebuffer to a normal client, so this
// backend only injects events. The absolute axis range is the screen size:
//
//	CLICK_SPIRIT_SCREEN=WxH     read by newUinput / platform.New
//	NewUinput(width, height)    explicit size, for callers that already know it
//
// Both forms describe one rectangle whose coordinates run from 0 to width-1
// and 0 to height-1. Multi-monitor layouts are not discovered here.
// Creating the device needs write access to /dev/uinput. A typical udev rule
// is KERNEL=="uinput", GROUP="input", MODE="0660", with the user in the input
// group. CursorPos returns ErrUinputCursor. This type does not capture.

const (
	evSyn = 0x00
	evKey = 0x01
	evAbs = 0x03

	synReport = 0x00

	absX = 0x00
	absY = 0x01

	btnLeft   = 0x110
	btnRight  = 0x111
	btnMiddle = 0x112

	busVirtual = 0x06

	// Values match linux/uinput.h (_IO / _IOW on 'U').
	uiDevCreate  = 0x5501
	uiDevDestroy = 0x5502
	uiDevSetup   = 0x405c5503
	uiAbsSetup   = 0x401c5504
	uiSetEvbit   = 0x40045564
	uiSetKeybit  = 0x40045565
	uiSetAbsbit  = 0x40045567

	uinputPath = "/dev/uinput"
)

// inputEvent matches the libc input_event on the arch that runs it.
// On amd64 and arm64, unix.Timeval is 16 bytes and the whole struct is 24.
// On 32-bit Linux, timeval is 8 bytes and the struct is 16.
type inputEvent struct {
	Time  unix.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// uinputSetup matches struct uinput_setup (92 bytes).
type uinputSetup struct {
	Bustype      uint16
	Vendor       uint16
	Product      uint16
	Version      uint16
	Name         [80]byte
	FFEffectsMax uint32
}

// uinputAbsSetup matches struct uinput_abs_setup (28 bytes, including padding).
type uinputAbsSetup struct {
	Code       uint16
	_          uint16
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

// uinputUserDev matches struct uinput_user_dev (1116 bytes, ABS_CNT == 64).
// It is the legacy write() path used when UI_DEV_SETUP returns EINVAL.
type uinputUserDev struct {
	Name         [80]byte
	Bustype      uint16
	Vendor       uint16
	Product      uint16
	Version      uint16
	FFEffectsMax uint32
	AbsMax       [64]int32
	AbsMin       [64]int32
	AbsFuzz      [64]int32
	AbsFlat      [64]int32
}

var (
	_ = ([1]byte{})[unsafe.Sizeof(uinputSetup{})-92]
	_ = ([1]byte{})[92-unsafe.Sizeof(uinputSetup{})]
	_ = ([1]byte{})[unsafe.Sizeof(uinputAbsSetup{})-28]
	_ = ([1]byte{})[28-unsafe.Sizeof(uinputAbsSetup{})]
	_ = ([1]byte{})[unsafe.Sizeof(uinputUserDev{})-1116]
	_ = ([1]byte{})[1116-unsafe.Sizeof(uinputUserDev{})]
)

// ErrUinputCursor is returned by the uinput backend's CursorPos.
var ErrUinputCursor = errors.New("uinput backend cannot read the cursor position")

type uinputClicker struct {
	mu     sync.Mutex
	fd     int
	width  int
	height int
	x, y   int
}

var (
	_ core.Clicker       = (*uinputClicker)(nil)
	_ core.CursorLocator = (*uinputClicker)(nil)
)

// newUinput reads CLICK_SPIRIT_SCREEN=WxH and opens /dev/uinput.
func newUinput() (core.Clicker, error) {
	s := strings.TrimSpace(os.Getenv("CLICK_SPIRIT_SCREEN"))
	if s == "" {
		return nil, errors.New("uinput: set CLICK_SPIRIT_SCREEN=WxH (Wayland cannot report the screen size)")
	}
	w, h, err := parseScreenSize(s)
	if err != nil {
		return nil, err
	}
	return NewUinput(w, h)
}

// parseScreenSize parses WxH. Width and height must be positive.
func parseScreenSize(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "x")
	if len(parts) != 2 {
		parts = strings.Split(s, "X")
	}
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("CLICK_SPIRIT_SCREEN must be WxH, got %q", s)
	}
	w, errW := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, errH := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("CLICK_SPIRIT_SCREEN must be positive WxH, got %q", s)
	}
	return w, h, nil
}

// NewUinput creates an absolute pointer covering [0, width) by [0, height).
// The device is destroyed by Close.
func NewUinput(width, height int) (core.Clicker, error) {
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("uinput screen size must be positive, got %dx%d", width, height)
	}
	fd, err := unix.Open(uinputPath, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w (add the user to the input group or install udev rule KERNEL==\"uinput\", GROUP=\"input\", MODE=\"0660\")", uinputPath, err)
	}
	u := &uinputClicker{fd: fd, width: width, height: height}
	if err := u.configure(); err != nil {
		_ = unix.IoctlSetInt(fd, uiDevDestroy, 0)
		_ = unix.Close(fd)
		u.fd = -1
		return nil, err
	}
	return u, nil
}

func (u *uinputClicker) configure() error {
	for _, bit := range []int{evSyn, evKey, evAbs} {
		if err := unix.IoctlSetInt(u.fd, uiSetEvbit, bit); err != nil {
			return fmt.Errorf("UI_SET_EVBIT %d: %w", bit, err)
		}
	}
	for _, key := range []int{btnLeft, btnRight, btnMiddle} {
		if err := unix.IoctlSetInt(u.fd, uiSetKeybit, key); err != nil {
			return fmt.Errorf("UI_SET_KEYBIT %d: %w", key, err)
		}
	}
	for _, axis := range []int{absX, absY} {
		if err := unix.IoctlSetInt(u.fd, uiSetAbsbit, axis); err != nil {
			return fmt.Errorf("UI_SET_ABSBIT %d: %w", axis, err)
		}
	}

	var setup uinputSetup
	setup.Bustype = busVirtual
	setup.Vendor = 0x0001
	setup.Product = 0x0001
	setup.Version = 1
	copy(setup.Name[:], "click-spirit")
	err := ioctlPtr(u.fd, uiDevSetup, unsafe.Pointer(&setup))
	if err != nil {
		if !isUnsupportedIoctl(err) {
			return fmt.Errorf("UI_DEV_SETUP: %w", err)
		}
		if err := writeLegacyUserDev(u.fd, u.width, u.height); err != nil {
			return err
		}
	} else {
		if err := setAbsAxis(u.fd, absX, u.width); err != nil {
			return err
		}
		if err := setAbsAxis(u.fd, absY, u.height); err != nil {
			return err
		}
	}
	if err := unix.IoctlSetInt(u.fd, uiDevCreate, 0); err != nil {
		return fmt.Errorf("UI_DEV_CREATE: %w", err)
	}
	// udev needs a moment before the new event node accepts writes.
	time.Sleep(20 * time.Millisecond)
	return nil
}

func setAbsAxis(fd, axis, span int) error {
	max := int32(span - 1)
	if max < 0 {
		max = 0
	}
	abs := uinputAbsSetup{
		Code:    uint16(axis),
		Maximum: max,
	}
	if err := ioctlPtr(fd, uiAbsSetup, unsafe.Pointer(&abs)); err != nil {
		return fmt.Errorf("UI_ABS_SETUP %d: %w", axis, err)
	}
	return nil
}

func writeLegacyUserDev(fd, width, height int) error {
	var dev uinputUserDev
	copy(dev.Name[:], "click-spirit")
	dev.Bustype = busVirtual
	dev.Version = 1
	dev.AbsMin[absX] = 0
	dev.AbsMax[absX] = int32(width - 1)
	dev.AbsMin[absY] = 0
	dev.AbsMax[absY] = int32(height - 1)
	if err := writeFull(fd, marshalStruct(unsafe.Pointer(&dev), unsafe.Sizeof(dev))); err != nil {
		return fmt.Errorf("write uinput_user_dev: %w", err)
	}
	return nil
}

// Close destroys the virtual device and closes /dev/uinput.
func (u *uinputClicker) Close() error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.fd < 0 {
		return nil
	}
	_ = unix.IoctlSetInt(u.fd, uiDevDestroy, 0)
	err := unix.Close(u.fd)
	u.fd = -1
	return err
}

func (u *uinputClicker) MoveTo(x, y int) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if err := u.emitAbs(x, y); err != nil {
		return err
	}
	u.x, u.y = x, y
	return nil
}

func (u *uinputClicker) Click(b core.Button, count, holdMs int) error {
	if count <= 0 {
		count = 1
	}
	code, err := uinputButton(b)
	if err != nil {
		return err
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	for i := 0; i < count; i++ {
		if err := u.emitAbs(u.x, u.y); err != nil {
			return err
		}
		if err := u.emit(evKey, code, 1); err != nil {
			return err
		}
		if err := u.syncReport(); err != nil {
			return err
		}
		if holdMs > 0 {
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
		}
		if err := u.emit(evKey, code, 0); err != nil {
			return err
		}
		if err := u.syncReport(); err != nil {
			return err
		}
	}
	return nil
}

// CursorPos is unsupported. Wayland does not give this device a pointer to read.
func (u *uinputClicker) CursorPos() (int, int, error) {
	return 0, 0, ErrUinputCursor
}

func (u *uinputClicker) emitAbs(x, y int) error {
	if err := u.emit(evAbs, absX, int32(x)); err != nil {
		return err
	}
	if err := u.emit(evAbs, absY, int32(y)); err != nil {
		return err
	}
	return u.syncReport()
}

func (u *uinputClicker) syncReport() error {
	return u.emit(evSyn, synReport, 0)
}

func (u *uinputClicker) emit(typ, code uint16, value int32) error {
	if u.fd < 0 {
		return errors.New("uinput device is closed")
	}
	ev := inputEvent{
		Time:  unix.NsecToTimeval(time.Now().UnixNano()),
		Type:  typ,
		Code:  code,
		Value: value,
	}
	return writeFull(u.fd, marshalInputEvent(ev))
}

func uinputButton(b core.Button) (uint16, error) {
	switch b {
	case core.ButtonLeft:
		return btnLeft, nil
	case core.ButtonRight:
		return btnRight, nil
	case core.ButtonMiddle:
		return btnMiddle, nil
	default:
		return 0, fmt.Errorf("invalid button %q", b)
	}
}

// marshalInputEvent serializes ev in native layout, which is the kernel's
// input_event layout on this machine. On amd64 the result is 24 bytes.
func marshalInputEvent(ev inputEvent) []byte {
	return marshalStruct(unsafe.Pointer(&ev), unsafe.Sizeof(ev))
}

func marshalStruct(ptr unsafe.Pointer, size uintptr) []byte {
	out := make([]byte, size)
	copy(out, unsafe.Slice((*byte)(ptr), size))
	return out
}

func writeFull(fd int, b []byte) error {
	for len(b) > 0 {
		n, err := unix.Write(fd, b)
		if n > 0 {
			b = b[n:]
			continue
		}
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EINTR) {
			time.Sleep(time.Millisecond)
			continue
		}
		if err != nil {
			return err
		}
		return errors.New("short write to uinput")
	}
	return nil
}

func ioctlPtr(fd int, req uint, p unsafe.Pointer) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(p))
	if errno != 0 {
		return errno
	}
	return nil
}

func isUnsupportedIoctl(err error) bool {
	return errors.Is(err, unix.EINVAL) || errors.Is(err, unix.ENOTTY) || errors.Is(err, unix.EOPNOTSUPP)
}
