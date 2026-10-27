//go:build linux

package platform

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/core/record"
)

// evdev is the Wayland (and non-XRecord) input path. It reads
// /dev/input/event* directly. Pointer coordinates are approximate:
// mice report relative motion, so the position is seeded from the X11
// backend's cursor when that backend is active and otherwise starts at
// 0,0. REL_X/REL_Y accumulate and are clamped to CLICK_SPIRIT_SCREEN
// when that size is known. Touchpads and tablets send EV_ABS; those
// values are scaled from the EVIOCGABS range onto the same screen.
// Without a known screen size the absolute axis is reported relative
// to its minimum, which is not a desktop pixel.
//
// Opening the device nodes requires the caller to be in the input group
// (or another reader of mode 0660 on /dev/input/event*). EACCES is
// returned wrapped with ErrUnsupported.

const (
	evRel = 0x02

	relX = 0x00
	relY = 0x01

	absMTX = 0x35
	absMTY = 0x36

	// evdevKeyBytes covers key codes 0..767 (KEY_MAX is 0x2ff).
	evdevKeyBytes = 96

	keyEsc        = 1
	key1          = 2
	key0          = 11
	keyBackspace  = 14
	keyTab        = 15
	keyQ          = 16
	keyEnter      = 28
	keyLeftCtrl   = 29
	keyA          = 30
	keyLeftShift  = 42
	keyZ          = 44
	keyRightShift = 54
	keyLeftAlt    = 56
	keySpace      = 57
	keyF1         = 59
	keyF11        = 87
	keyF12        = 88
	keyRightCtrl  = 97
	keyRightAlt   = 100
	keyHome       = 102
	keyUp         = 103
	keyPageUp     = 104
	keyLeft       = 105
	keyRight      = 106
	keyEnd        = 107
	keyDown       = 108
	keyPageDown   = 109
	keyInsert     = 110
	keyDelete     = 111
	keyLeftMeta   = 125
	keyRightMeta  = 126
	keyF13        = 183
)

// inputAbsInfo matches struct input_absinfo (6 little-endian int32s).
type inputAbsInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

var (
	_ = ([1]byte{})[unsafe.Sizeof(inputAbsInfo{})-24]
	_ = ([1]byte{})[24-unsafe.Sizeof(inputAbsInfo{})]
)

func errNeedInputGroup() error {
	return fmt.Errorf("cannot read /dev/input/event* (add the user to the input group): %w", ErrUnsupported)
}

// bitSet reports whether bit is set in an EVIOCGBIT mask.
func bitSet(bits []byte, bit int) bool {
	if bit < 0 {
		return false
	}
	i := bit / 8
	if i >= len(bits) {
		return false
	}
	return bits[i]&(1<<uint(bit%8)) != 0
}

func pointerKeyBits(bits []byte) bool {
	return bitSet(bits, int(btnLeft))
}

func keyboardKeyBits(bits []byte) bool {
	for code := keyF1; code <= keyF1+9; code++ {
		if bitSet(bits, code) {
			return true
		}
	}
	return bitSet(bits, keyF11) || bitSet(bits, keyF12)
}

// evdevIOC builds an _IOC read/write number for the 'E' input family.
// dir is 2 for _IOC_READ. nr and size match linux/ioctl.h.
func evdevIOC(dir, nr, size int) uint {
	return uint(dir<<30) | uint(size<<16) | uint('E')<<8 | uint(nr)
}

func evdevBitReq(evType, size int) uint {
	return evdevIOC(2, 0x20+evType, size)
}

func evdevAbsReq(axis int) uint {
	return evdevIOC(2, 0x40+axis, int(unsafe.Sizeof(inputAbsInfo{})))
}

func ioctlRead(fd int, req uint, buf []byte) error {
	if len(buf) == 0 {
		return errors.New("empty ioctl buffer")
	}
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), uintptr(unsafe.Pointer(&buf[0])))
	if errno != 0 {
		return errno
	}
	return nil
}

func readAbsInfo(fd, axis int) (inputAbsInfo, error) {
	var info inputAbsInfo
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), uintptr(evdevAbsReq(axis)), uintptr(unsafe.Pointer(&info)))
	if errno != 0 {
		return info, errno
	}
	return info, nil
}

type absAxis struct {
	min int
	max int
}

func readAbsAxis(fd, axis int) (absAxis, bool) {
	info, err := readAbsInfo(fd, axis)
	if err != nil || info.Maximum <= info.Minimum {
		return absAxis{}, false
	}
	return absAxis{min: int(info.Minimum), max: int(info.Maximum)}, true
}

type evdevDev struct {
	fd     int
	absX   absAxis
	absY   absAxis
	absMTX absAxis
	absMTY absAxis
	hasX   bool
	hasY   bool
	hasMTX bool
	hasMTY bool
	rest   []byte
}

func (d *evdevDev) close() {
	if d != nil && d.fd >= 0 {
		_ = unix.Close(d.fd)
		d.fd = -1
	}
}

func (d *evdevDev) loadAbs() {
	if ax, ok := readAbsAxis(d.fd, absX); ok {
		d.absX, d.hasX = ax, true
	}
	if ax, ok := readAbsAxis(d.fd, absY); ok {
		d.absY, d.hasY = ax, true
	}
	if ax, ok := readAbsAxis(d.fd, absMTX); ok {
		d.absMTX, d.hasMTX = ax, true
	}
	if ax, ok := readAbsAxis(d.fd, absMTY); ok {
		d.absMTY, d.hasMTY = ax, true
	}
}

func (d *evdevDev) readEvents() ([]inputEvent, error) {
	sz := int(unsafe.Sizeof(inputEvent{}))
	if sz <= 0 {
		return nil, errors.New("input_event size is zero")
	}
	buf := make([]byte, 64*sz)
	for {
		n, err := unix.Read(d.fd, buf)
		if n > 0 {
			d.rest = append(d.rest, buf[:n]...)
		}
		if err != nil {
			if errors.Is(err, unix.EAGAIN) || errors.Is(err, unix.EWOULDBLOCK) {
				break
			}
			if errors.Is(err, unix.EINTR) {
				continue
			}
			return nil, err
		}
		if n == 0 || n < len(buf) {
			break
		}
	}
	evs, rest := parseInputEventBytes(d.rest)
	d.rest = rest
	return evs, nil
}

// parseInputEventBytes splits a native input_event stream.
// A trailing partial event is returned in rest and must be kept for the
// next read. The layout matches marshalInputEvent / uinput_event_size_64.go:
// on amd64 and arm64 each event is 24 bytes.
func parseInputEventBytes(b []byte) (evs []inputEvent, rest []byte) {
	sz := int(unsafe.Sizeof(inputEvent{}))
	if sz <= 0 || len(b) < sz {
		return nil, append([]byte(nil), b...)
	}
	n := len(b) / sz
	evs = make([]inputEvent, n)
	for i := 0; i < n; i++ {
		copy(unsafe.Slice((*byte)(unsafe.Pointer(&evs[i])), sz), b[i*sz:(i+1)*sz])
	}
	return evs, append([]byte(nil), b[n*sz:]...)
}

func openEvdevDevices(want func([]byte) bool, withAbs bool) ([]*evdevDev, error) {
	entries, err := os.ReadDir("/dev/input")
	if err != nil {
		if errors.Is(err, unix.EACCES) || errors.Is(err, os.ErrPermission) {
			return nil, errNeedInputGroup()
		}
		return nil, fmt.Errorf("read /dev/input: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "event") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var devs []*evdevDev
	sawDeny := false
	for _, name := range names {
		fd, err := unix.Open("/dev/input/"+name, unix.O_RDONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err != nil {
			if errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) {
				sawDeny = true
			}
			continue
		}
		bits := make([]byte, evdevKeyBytes)
		if err := ioctlRead(fd, evdevBitReq(evKey, len(bits)), bits); err != nil || !want(bits) {
			_ = unix.Close(fd)
			continue
		}
		dev := &evdevDev{fd: fd}
		if withAbs {
			dev.loadAbs()
		}
		devs = append(devs, dev)
	}
	if len(devs) == 0 && sawDeny {
		return nil, errNeedInputGroup()
	}
	return devs, nil
}

func closeEvdevDevs(devs []*evdevDev) {
	for _, d := range devs {
		d.close()
	}
}

// scaleAbs maps value from the closed device range [min, max] onto
// [0, screen-1]. screen <= 1 means the desktop size is unknown: the
// result is value-min clamped at 0 (the device minimum sits at the origin).
func scaleAbs(value, min, max, screen int) int {
	if value < min {
		value = min
	}
	if max > min && value > max {
		value = max
	}
	if screen <= 1 {
		v := value - min
		if v < 0 {
			return 0
		}
		return v
	}
	if max <= min {
		return 0
	}
	return (value - min) * (screen - 1) / (max - min)
}

// clampToScreen keeps x,y at >= 0. When w or h is positive the matching
// axis is also kept inside [0, size-1]. A non-positive size means that
// axis has no known screen edge.
func clampToScreen(x, y, w, h int) (int, int) {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if w > 0 && x >= w {
		x = w - 1
	}
	if h > 0 && y >= h {
		y = h - 1
	}
	return x, y
}

type pointerTrack struct {
	x, y int
	w, h int
}

func (p *pointerTrack) move() record.Event {
	return record.Event{Kind: record.KindMove, X: p.x, Y: p.y}
}

// applyEvdevPointer folds one input_event into the tracked cursor.
// The bool is false for sync, scroll, and keys that are not buttons.
// Time is left zero; the caller sets time.Now at delivery.
func applyEvdevPointer(p *pointerTrack, dev *evdevDev, ev inputEvent) (record.Event, bool) {
	switch ev.Type {
	case evRel:
		switch ev.Code {
		case relX:
			p.x, p.y = clampToScreen(p.x+int(ev.Value), p.y, p.w, p.h)
			return p.move(), true
		case relY:
			p.x, p.y = clampToScreen(p.x, p.y+int(ev.Value), p.w, p.h)
			return p.move(), true
		default:
			return record.Event{}, false
		}
	case evAbs:
		return applyAbs(p, dev, ev)
	case evKey:
		button, ok := evdevButton(ev.Code)
		if !ok {
			return record.Event{}, false
		}
		kind := record.KindUp
		switch ev.Value {
		case 1:
			kind = record.KindDown
		case 0:
		default:
			return record.Event{}, false
		}
		return record.Event{Kind: kind, Button: button, X: p.x, Y: p.y}, true
	default:
		return record.Event{}, false
	}
}

func applyAbs(p *pointerTrack, dev *evdevDev, ev inputEvent) (record.Event, bool) {
	if dev == nil {
		return record.Event{}, false
	}
	var (
		axis   absAxis
		has    bool
		screen int
		xAxis  bool
	)
	switch ev.Code {
	case absX:
		axis, has, screen, xAxis = dev.absX, dev.hasX, p.w, true
	case absY:
		axis, has, screen, xAxis = dev.absY, dev.hasY, p.h, false
	case absMTX:
		axis, has, screen, xAxis = dev.absMTX, dev.hasMTX, p.w, true
	case absMTY:
		axis, has, screen, xAxis = dev.absMTY, dev.hasMTY, p.h, false
	default:
		return record.Event{}, false
	}
	var v int
	if has {
		v = scaleAbs(int(ev.Value), axis.min, axis.max, screen)
	} else if ev.Value > 0 {
		v = int(ev.Value)
	}
	if xAxis {
		p.x = v
	} else {
		p.y = v
	}
	p.x, p.y = clampToScreen(p.x, p.y, p.w, p.h)
	return p.move(), true
}

func evdevButton(code uint16) (core.Button, bool) {
	switch code {
	case btnLeft:
		return core.ButtonLeft, true
	case btnMiddle:
		return core.ButtonMiddle, true
	case btnRight:
		return core.ButtonRight, true
	default:
		return "", false
	}
}

// evdevKeyCode maps a canonical hotkey name (core.ParseHotkey) to a Linux
// key code. The bool is false when the name has no evdev equivalent.
func evdevKeyCode(name string) (uint16, bool) {
	if len(name) >= 2 && name[0] == 'f' {
		n := 0
		for _, c := range name[1:] {
			if c < '0' || c > '9' {
				n = -1
				break
			}
			n = n*10 + int(c-'0')
		}
		switch {
		case n >= 1 && n <= 10:
			return uint16(keyF1 + n - 1), true
		case n == 11 || n == 12:
			return uint16(keyF11 + n - 11), true
		case n >= 13 && n <= 24:
			return uint16(keyF13 + n - 13), true
		}
	}
	if len(name) == 1 {
		if code, ok := letterKey(name[0]); ok {
			return code, true
		}
	}
	code, ok := namedEvdevKeys[name]
	return code, ok
}

func letterKey(c byte) (uint16, bool) {
	switch c {
	case 'q':
		return keyQ, true
	case 'w':
		return 17, true
	case 'e':
		return 18, true
	case 'r':
		return 19, true
	case 't':
		return 20, true
	case 'y':
		return 21, true
	case 'u':
		return 22, true
	case 'i':
		return 23, true
	case 'o':
		return 24, true
	case 'p':
		return 25, true
	case 'a':
		return keyA, true
	case 's':
		return 31, true
	case 'd':
		return 32, true
	case 'f':
		return 33, true
	case 'g':
		return 34, true
	case 'h':
		return 35, true
	case 'j':
		return 36, true
	case 'k':
		return 37, true
	case 'l':
		return 38, true
	case 'z':
		return keyZ, true
	case 'x':
		return 45, true
	case 'c':
		return 46, true
	case 'v':
		return 47, true
	case 'b':
		return 48, true
	case 'n':
		return 49, true
	case 'm':
		return 50, true
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		return uint16(key1 + int(c-'1')), true
	case '0':
		return key0, true
	default:
		return 0, false
	}
}

var namedEvdevKeys = map[string]uint16{
	"escape":    keyEsc,
	"return":    keyEnter,
	"tab":       keyTab,
	"backspace": keyBackspace,
	"delete":    keyDelete,
	"insert":    keyInsert,
	"home":      keyHome,
	"end":       keyEnd,
	"pageup":    keyPageUp,
	"pagedown":  keyPageDown,
	"left":      keyLeft,
	"up":        keyUp,
	"right":     keyRight,
	"down":      keyDown,
	"space":     keySpace,
}

// modifierState tracks both sides of each chord modifier.
// Releasing the left key while the right key is held keeps the modifier down.
type modifierState struct {
	ctrlL, ctrlR   bool
	shiftL, shiftR bool
	altL, altR     bool
	superL, superR bool
}

func (m modifierState) ctrl() bool  { return m.ctrlL || m.ctrlR }
func (m modifierState) shift() bool { return m.shiftL || m.shiftR }
func (m modifierState) alt() bool   { return m.altL || m.altR }
func (m modifierState) super() bool { return m.superL || m.superR }

func (m *modifierState) apply(code uint16, value int32) {
	down := value != 0
	switch code {
	case keyLeftCtrl:
		m.ctrlL = down
	case keyRightCtrl:
		m.ctrlR = down
	case keyLeftShift:
		m.shiftL = down
	case keyRightShift:
		m.shiftR = down
	case keyLeftAlt:
		m.altL = down
	case keyRightAlt:
		m.altR = down
	case keyLeftMeta:
		m.superL = down
	case keyRightMeta:
		m.superR = down
	}
}

// chordHit reports whether this evdev key event completes chord.
// value 1 is a press. Repeats (2) and releases (0) do not fire.
// Modifiers must match exactly, the same way XGrabKey uses one mask.
func chordHit(chord core.Chord, code uint16, value int32, mods modifierState) bool {
	if value != 1 {
		return false
	}
	want, ok := evdevKeyCode(chord.Key)
	if !ok || want != code {
		return false
	}
	return mods.ctrl() == chord.Ctrl && mods.shift() == chord.Shift && mods.alt() == chord.Alt && mods.super() == chord.Super
}

func pollFDs(fds []int, wake int) error {
	pf := make([]unix.PollFd, 0, len(fds)+1)
	for _, fd := range fds {
		if fd >= 0 {
			pf = append(pf, unix.PollFd{Fd: int32(fd), Events: unix.POLLIN})
		}
	}
	wakeAt := len(pf)
	if wake >= 0 {
		pf = append(pf, unix.PollFd{Fd: int32(wake), Events: unix.POLLIN})
	}
	if len(pf) == 0 {
		return errors.New("no fds to poll")
	}
	for {
		n, err := unix.Poll(pf, 200)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if wakeAt < len(pf) && pf[wakeAt].Revents != 0 {
			return errors.New("evdev wake")
		}
		if n == 0 {
			return nil
		}
		for i := 0; i < wakeAt; i++ {
			if pf[i].Revents != 0 {
				return nil
			}
		}
	}
}

// evdevInput records pointer devices (EV_KEY + BTN_LEFT).
// Stop is idempotent. A later Start needs a new value from NewInputSource.
// The callback runs on the event-loop goroutine and must not call Stop.
type evdevInput struct {
	mu      sync.Mutex
	wakeR   int
	wakeW   int
	running bool
	exited  chan struct{}
	stopReq atomic.Bool
}

func newEvdevInputSource() (record.InputSource, error) {
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return nil, err
	}
	return &evdevInput{wakeR: p[0], wakeW: p[1]}, nil
}

func (s *evdevInput) Start(fn func(record.Event)) error {
	if fn == nil {
		return errors.New("nil record callback")
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return errors.New("recording already started")
	}
	if s.stopReq.Load() {
		s.mu.Unlock()
		return errors.New("recording stopped")
	}
	exited := make(chan struct{})
	s.exited = exited
	s.running = true
	s.mu.Unlock()

	errCh := make(chan error, 1)
	go s.loop(fn, errCh, exited)
	if err := <-errCh; err != nil {
		<-exited
		s.mu.Lock()
		s.running = false
		s.exited = nil
		s.mu.Unlock()
		s.closePipe()
		return err
	}
	return nil
}

func (s *evdevInput) Stop() {
	s.stopReq.Store(true)
	s.wake()
	s.mu.Lock()
	exited := s.exited
	s.mu.Unlock()
	if exited != nil {
		<-exited
	}
	s.closePipe()
}

func (s *evdevInput) wake() {
	s.mu.Lock()
	w := s.wakeW
	s.mu.Unlock()
	if w >= 0 {
		_, _ = unix.Write(w, []byte{1})
	}
}

func (s *evdevInput) closePipe() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wakeR >= 0 {
		_ = unix.Close(s.wakeR)
		s.wakeR = -1
	}
	if s.wakeW >= 0 {
		_ = unix.Close(s.wakeW)
		s.wakeW = -1
	}
}

func (s *evdevInput) loop(fn func(record.Event), errCh chan<- error, exited chan struct{}) {
	defer close(exited)
	var sent bool
	signal := func(err error) {
		if sent {
			return
		}
		sent = true
		errCh <- err
	}
	defer func() {
		signal(errors.New("record loop exited before it was armed"))
	}()

	s.mu.Lock()
	wake := s.wakeR
	s.mu.Unlock()
	if wake < 0 || s.stopReq.Load() {
		signal(errors.New("recording stopped"))
		return
	}
	devs, err := openEvdevDevices(pointerKeyBits, true)
	if err != nil {
		signal(err)
		return
	}
	defer closeEvdevDevs(devs)
	if len(devs) == 0 {
		signal(fmt.Errorf("no pointer device with BTN_LEFT in /dev/input: %w", ErrUnsupported))
		return
	}
	x, y, w, h := pointerSeed()
	track := pointerTrack{x: x, y: y, w: w, h: h}
	signal(nil)

	fds := make([]int, len(devs))
	for i, d := range devs {
		fds[i] = d.fd
	}
	for {
		if s.stopReq.Load() {
			return
		}
		for _, d := range devs {
			evs, rerr := d.readEvents()
			if rerr != nil {
				return
			}
			for _, ev := range evs {
				got, ok := applyEvdevPointer(&track, d, ev)
				if !ok {
					continue
				}
				got.Time = time.Now()
				fn(got)
			}
		}
		if err := pollFDs(fds, wake); err != nil {
			return
		}
	}
}
