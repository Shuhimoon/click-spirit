//go:build linux && cgo && !nox11

package platform

/*
#cgo pkg-config: x11 xtst

#include <stdint.h>
#include <unistd.h>
#include <X11/Xlib.h>
#include <X11/extensions/record.h>

extern void csRecordIntercept(uintptr_t handle, void *data);

static void csRecordThunk(XPointer closure, XRecordInterceptData *data) {
	csRecordIntercept((uintptr_t)closure, data);
}

static int csRecordVersion(Display *dpy) {
	int major = 0, minor = 0;
	if (dpy == NULL) {
		return 0;
	}
	return XRecordQueryVersion(dpy, &major, &minor);
}

static XRecordContext csRecordCreate(Display *dpy) {
	XRecordRange *range;
	XRecordClientSpec clients;
	XRecordContext ctx;
	if (dpy == NULL) {
		return 0;
	}
	range = XRecordAllocRange();
	if (range == NULL) {
		return 0;
	}
	range->device_events.first = ButtonPress;
	range->device_events.last = MotionNotify;
	clients = XRecordAllClients;
	ctx = XRecordCreateContext(dpy, 0, &clients, 1, &range, 1);
	XFree(range);
	return ctx;
}

static int csRecordEnable(Display *dpy, XRecordContext ctx, uintptr_t handle) {
	Status st;
	if (dpy == NULL || ctx == 0) {
		return 0;
	}
	st = XRecordEnableContextAsync(dpy, ctx, csRecordThunk, (XPointer)handle);
	XFlush(dpy);
	return st ? 1 : 0;
}

static void csRecordDisable(Display *dpy, XRecordContext ctx) {
	if (dpy == NULL || ctx == 0) {
		return;
	}
	XRecordDisableContext(dpy, ctx);
	XFlush(dpy);
}

static void csRecordFree(Display *dpy, XRecordContext ctx) {
	if (dpy == NULL || ctx == 0) {
		return;
	}
	XRecordFreeContext(dpy, ctx);
}

static void csRecordProcess(Display *dpy) {
	if (dpy != NULL) {
		XRecordProcessReplies(dpy);
	}
}

static int csScreenSize(Display *dpy, int *w, int *h) {
	int screen;
	if (dpy == NULL || w == NULL || h == NULL) {
		return -1;
	}
	screen = XDefaultScreen(dpy);
	*w = XDisplayWidth(dpy, screen);
	*h = XDisplayHeight(dpy, screen);
	if (*w <= 0 || *h <= 0) {
		return -1;
	}
	return 0;
}

// A file with //export only sees its own preamble, so these wrappers are
// local copies of the clicker and hotkey helpers.
static int cs_rec_err;
static XErrorHandler cs_rec_prev;

static int csRecErrHandler(Display *dpy, XErrorEvent *ev) {
	(void)dpy;
	if (ev != NULL) {
		cs_rec_err = (int)ev->error_code;
	}
	return 0;
}

static void csRecResetErr(void) { cs_rec_err = 0; }
static int csRecErr(void) { return cs_rec_err; }
static void csRecInstall(void) { cs_rec_prev = XSetErrorHandler(csRecErrHandler); }
static void csRecRestore(void) { XSetErrorHandler(cs_rec_prev); }

// Same transient Xvfb connect failures as csOpen. This file exports a Go
// callback, so it cannot call static helpers defined in the other X11 files.
static Display *csRecOpen(void) {
	int i;
	for (i = 0; i < 20; i++) {
		Display *d = XOpenDisplay(NULL);
		if (d != NULL || i == 19) {
			return d;
		}
		usleep(1000);
	}
	return NULL;
}

static void csRecClose(Display *dpy) {
	if (dpy != NULL) {
		XCloseDisplay(dpy);
	}
}

static void csRecSync(Display *dpy) {
	if (dpy != NULL) {
		XSync(dpy, False);
	}
}

static int csRecConn(Display *dpy) {
	if (dpy == NULL) {
		return -1;
	}
	return XConnectionNumber(dpy);
}

static int csRecCursor(Display *dpy, int *x, int *y) {
	Window root, rootRet, childRet;
	int rx = 0, ry = 0, wx = 0, wy = 0;
	unsigned int mask = 0;
	if (dpy == NULL || x == NULL || y == NULL) {
		return -1;
	}
	root = XDefaultRootWindow(dpy);
	if (!XQueryPointer(dpy, root, &rootRet, &childRet, &rx, &ry, &wx, &wy, &mask)) {
		return -1;
	}
	*x = rx;
	*y = ry;
	return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/cgo"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/click-spirit/click-spirit/core/record"
)

// x11InputSource records core pointer events with the X RECORD extension.
//
// Two Display connections are required: the control connection creates and
// disables the context, and the data connection runs XRecordEnableContextAsync.
// The data connection is polled with the same wake-pipe pattern as the X11
// hotkey. Stop is idempotent. Start after Stop needs a new NewInputSource.
//
// The callback runs on the event-loop goroutine and must not call Stop.
// Timestamps are time.Now at delivery so ordering stays on Go's monotonic clock.
//
// Events synthesized by our own XTest playback (XTestFakeMotionEvent and
// XTestFakeButtonEvent) are delivered like any other client's device events.
// They are not filtered. Recording during playback captures those clicks,
// and the Xvfb test depends on that.
type x11InputSource struct {
	mu       sync.Mutex
	wakeR    int
	wakeW    int
	running  bool
	exited   chan struct{}
	stopReq  atomic.Bool
	sawEnd   atomic.Bool
	enabled  bool
	disabled bool

	fn   func(record.Event)
	ctrl *C.Display
	data *C.Display
	ctx  C.XRecordContext

	// handle is the cgo.Handle passed to XRecordEnableContextAsync.
	// XCloseDisplay on the data connection can still invoke the intercept,
	// so the handle stays live until that display (and the control display)
	// have been closed. handleLive is false before NewHandle and after Delete.
	handle     cgo.Handle
	handleLive bool
}

func newX11InputSource() (record.InputSource, error) {
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return nil, err
	}
	return &x11InputSource{wakeR: p[0], wakeW: p[1]}, nil
}

// x11RecordAvailable is true when chooseBackend selects X11 and the server
// has the RECORD extension. It does not open a display unless both the
// backend is X11 and DISPLAY is set.
func x11RecordAvailable() bool {
	name, err := chooseBackend()
	if err != nil || name != "x11" {
		return false
	}
	if os.Getenv("DISPLAY") == "" {
		return false
	}
	dpy := C.csRecOpen()
	if dpy == nil {
		return false
	}
	defer C.csRecClose(dpy)
	return x11DisplayHasRecord(dpy)
}

func x11DisplayHasRecord(dpy *C.Display) bool {
	if dpy == nil {
		return false
	}
	xGrabMu.Lock()
	defer xGrabMu.Unlock()
	C.csRecResetErr()
	C.csRecInstall()
	defer C.csRecRestore()
	ok := C.csRecordVersion(dpy) != 0
	C.csRecSync(dpy)
	return ok && C.csRecErr() == 0
}

// x11PointerInfo reads the pointer and the root window size. ok is false
// when DISPLAY cannot be opened or the pointer query fails.
func x11PointerInfo() (x, y, w, h int, ok bool) {
	if os.Getenv("DISPLAY") == "" {
		return 0, 0, 0, 0, false
	}
	dpy := C.csRecOpen()
	if dpy == nil {
		return 0, 0, 0, 0, false
	}
	defer C.csRecClose(dpy)
	var cx, cy C.int
	if C.csRecCursor(dpy, &cx, &cy) != 0 {
		return 0, 0, 0, 0, false
	}
	var sw, sh C.int
	if C.csScreenSize(dpy, &sw, &sh) != 0 {
		return int(cx), int(cy), 0, 0, true
	}
	return int(cx), int(cy), int(sw), int(sh), true
}

func (s *x11InputSource) Start(fn func(record.Event)) error {
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
		s.freeAndClose()
		s.closePipe()
		return err
	}
	return nil
}

func (s *x11InputSource) Stop() {
	s.stopReq.Store(true)
	s.disable()
	s.wake()
	s.mu.Lock()
	exited := s.exited
	s.mu.Unlock()
	if exited != nil {
		<-exited
	}
	s.freeAndClose()
	s.closePipe()
}

func (s *x11InputSource) wake() {
	s.mu.Lock()
	w := s.wakeW
	s.mu.Unlock()
	if w >= 0 {
		_, _ = unix.Write(w, []byte{1})
	}
}

func (s *x11InputSource) closePipe() {
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

func (s *x11InputSource) disable() {
	s.mu.Lock()
	if s.disabled || !s.enabled {
		s.mu.Unlock()
		return
	}
	s.disabled = true
	ctrl := s.ctrl
	ctx := s.ctx
	s.mu.Unlock()
	if ctrl == nil || ctx == 0 {
		return
	}
	xGrabMu.Lock()
	C.csRecResetErr()
	C.csRecInstall()
	C.csRecordDisable(ctrl, ctx)
	C.csRecRestore()
	xGrabMu.Unlock()
}

func (s *x11InputSource) freeAndClose() {
	s.disable()
	s.mu.Lock()
	ctrl, data := s.ctrl, s.data
	ctx := s.ctx
	s.ctrl, s.data, s.ctx = nil, nil, 0
	s.mu.Unlock()
	if ctrl != nil && ctx != 0 {
		xGrabMu.Lock()
		C.csRecResetErr()
		C.csRecInstall()
		C.csRecordFree(ctrl, ctx)
		C.csRecSync(ctrl)
		C.csRecRestore()
		xGrabMu.Unlock()
	}
	// The loop normally closes the data display itself, before this runs,
	// so the intercept still has a live handle. This closes it if the loop
	// never reached that point.
	if data != nil {
		C.csRecordProcess(data)
		C.csRecClose(data)
	}
	if ctrl != nil {
		C.csRecClose(ctrl)
	}
	s.dropHandle()
}

// closeDataDisplay runs on the record loop's OS thread. XCloseDisplay can
// dispatch a last intercept, so the cgo.Handle must still be live here.
func (s *x11InputSource) closeDataDisplay() {
	s.mu.Lock()
	data := s.data
	s.data = nil
	s.mu.Unlock()
	if data == nil {
		return
	}
	C.csRecordProcess(data)
	C.csRecClose(data)
}

func (s *x11InputSource) dropHandle() {
	s.mu.Lock()
	live := s.handleLive
	h := s.handle
	s.handleLive = false
	s.mu.Unlock()
	if live {
		h.Delete()
	}
}

func (s *x11InputSource) loop(fn func(record.Event), errCh chan<- error, exited chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer close(exited)
	// Close the data display before Stop proceeds, on this thread, and
	// before UnlockOSThread. Defer runs last-in first-out.
	defer s.closeDataDisplay()
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

	ctrl := C.csRecOpen()
	if ctrl == nil {
		signal(fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY")))
		return
	}
	s.mu.Lock()
	s.ctrl = ctrl
	s.mu.Unlock()

	data := C.csRecOpen()
	if data == nil {
		signal(fmt.Errorf("XOpenDisplay failed for the XRecord data connection (DISPLAY=%q)", os.Getenv("DISPLAY")))
		return
	}
	s.mu.Lock()
	s.data = data
	s.mu.Unlock()

	if !x11DisplayHasRecord(ctrl) {
		signal(fmt.Errorf("XRecord extension is not available: %w", ErrUnsupported))
		return
	}

	xGrabMu.Lock()
	C.csRecResetErr()
	C.csRecInstall()
	ctx := C.csRecordCreate(ctrl)
	C.csRecSync(ctrl)
	createErr := int(C.csRecErr())
	C.csRecRestore()
	xGrabMu.Unlock()
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	if ctx == 0 || createErr != 0 {
		signal(fmt.Errorf("XRecordCreateContext failed (X error %d)", createErr))
		return
	}

	s.mu.Lock()
	s.fn = fn
	s.mu.Unlock()
	h := cgo.NewHandle(s)
	s.mu.Lock()
	s.handle = h
	s.handleLive = true
	s.mu.Unlock()
	if C.csRecordEnable(data, ctx, C.uintptr_t(uintptr(h))) == 0 {
		signal(errors.New("XRecordEnableContextAsync failed"))
		return
	}
	s.mu.Lock()
	s.enabled = true
	s.mu.Unlock()
	if s.stopReq.Load() {
		s.disable()
		signal(errors.New("recording stopped"))
		return
	}
	xfd := int(C.csRecConn(data))
	if xfd < 0 {
		s.disable()
		signal(errors.New("XConnectionNumber failed"))
		return
	}
	signal(nil)

	for {
		stopping := s.stopReq.Load() || s.sawEnd.Load()
		C.csRecordProcess(data)
		if stopping || s.sawEnd.Load() {
			return
		}
		if err := pollX(xfd, wake); err != nil {
			C.csRecordProcess(data)
			return
		}
	}
}

//export csRecordIntercept
func csRecordIntercept(handle C.uintptr_t, raw unsafe.Pointer) {
	if raw == nil {
		if src, ok := recordSource(handle); ok {
			src.sawEnd.Store(true)
		}
		return
	}
	d := (*C.XRecordInterceptData)(raw)
	defer C.XRecordFreeData(d)
	src, ok := recordSource(handle)
	if !ok {
		return
	}
	src.mu.Lock()
	fn := src.fn
	src.mu.Unlock()
	if fn == nil {
		return
	}
	cat, payload, swap := xRecordPayload(d)
	if cat != int(C.XRecordFromServer) {
		return
	}
	for _, ev := range parseXRecordServerData(payload, swap) {
		ev.Time = time.Now()
		fn(ev)
	}
}

func recordSource(handle C.uintptr_t) (*x11InputSource, bool) {
	v := cgo.Handle(uintptr(handle)).Value()
	src, ok := v.(*x11InputSource)
	return src, ok
}

func xRecordPayload(d *C.XRecordInterceptData) (cat int, payload []byte, swap bool) {
	if d == nil {
		return -1, nil, false
	}
	cat = int(d.category)
	swap = d.client_swapped != 0
	if d.data == nil || d.data_len == 0 {
		return cat, nil, swap
	}
	units := uint64(d.data_len)
	if units > 1<<18 {
		units = 1 << 18
	}
	n := units * 4
	if n == 0 || n > 1<<20 {
		return cat, nil, swap
	}
	return cat, C.GoBytes(unsafe.Pointer(d.data), C.int(n)), swap
}
