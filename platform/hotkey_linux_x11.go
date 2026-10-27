//go:build linux && cgo && !nox11

package platform

/*
#cgo pkg-config: x11 xtst
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/extensions/XTest.h>

static Display *hkOpen(void) { return XOpenDisplay(NULL); }

static void hkClose(Display *dpy) {
	if (dpy != NULL) {
		XCloseDisplay(dpy);
	}
}

static int hkXTest(Display *dpy) {
	int eventBase = 0, errorBase = 0, major = 0, minor = 0;
	if (dpy == NULL) {
		return 0;
	}
	return XTestQueryExtension(dpy, &eventBase, &errorBase, &major, &minor);
}

static int cs_hotkey_err;
static XErrorHandler cs_hotkey_prev;

static int csHotkeyErrHandler(Display *dpy, XErrorEvent *ev) {
	(void)dpy;
	if (ev != NULL) {
		cs_hotkey_err = (int)ev->error_code;
	}
	return 0;
}

static void csHotkeyResetErr(void) { cs_hotkey_err = 0; }
static int csHotkeyErr(void) { return cs_hotkey_err; }

static void csHotkeyInstallHandler(void) {
	cs_hotkey_prev = XSetErrorHandler(csHotkeyErrHandler);
}

static void csHotkeyRestoreHandler(void) {
	XSetErrorHandler(cs_hotkey_prev);
}

static int csKeycode(Display *dpy, unsigned long sym) {
	if (dpy == NULL) {
		return 0;
	}
	return (int)XKeysymToKeycode(dpy, (KeySym)sym);
}

static int csConn(Display *dpy) {
	if (dpy == NULL) {
		return -1;
	}
	return XConnectionNumber(dpy);
}

static void csSelectKeys(Display *dpy) {
	if (dpy == NULL) {
		return;
	}
	XSelectInput(dpy, XDefaultRootWindow(dpy), KeyPressMask);
}

static void csGrabKey(Display *dpy, int keycode, unsigned int mods) {
	if (dpy == NULL) {
		return;
	}
	XGrabKey(dpy, keycode, mods, XDefaultRootWindow(dpy), False, GrabModeAsync, GrabModeAsync);
}

static void csUngrabKey(Display *dpy, int keycode, unsigned int mods) {
	if (dpy == NULL) {
		return;
	}
	XUngrabKey(dpy, keycode, mods, XDefaultRootWindow(dpy));
}

static void csSync(Display *dpy) {
	if (dpy != NULL) {
		XSync(dpy, False);
	}
}

static int csPending(Display *dpy) {
	if (dpy == NULL) {
		return 0;
	}
	return XPending(dpy);
}

static int csTakeKey(Display *dpy, int keycode) {
	if (dpy == NULL || !XPending(dpy)) {
		return 0;
	}
	XEvent ev;
	XNextEvent(dpy, &ev);
	if (ev.type == KeyPress && (int)ev.xkey.keycode == keycode) {
		return 1;
	}
	return 0;
}

static int hkFocusRoot(Display *dpy) {
	if (dpy == NULL) {
		return -1;
	}
	XSetInputFocus(dpy, XDefaultRootWindow(dpy), RevertToParent, CurrentTime);
	XFlush(dpy);
	XSync(dpy, False);
	return 0;
}

static int csTapKey(Display *dpy, unsigned int keycode) {
	if (dpy == NULL) {
		return -1;
	}
	if (!XTestFakeKeyEvent(dpy, keycode, True, 0)) {
		return -1;
	}
	if (!XTestFakeKeyEvent(dpy, keycode, False, 0)) {
		return -1;
	}
	XFlush(dpy);
	XSync(dpy, False);
	return 0;
}
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"

	"github.com/click-spirit/click-spirit/core"
)

// x11Hotkey grabs one chord on its own Display connection and waits for
// KeyPress on a goroutine. Stop wakes that goroutine through a pipe.
// The callback runs on the event-loop goroutine and must not call Stop.
type x11Hotkey struct {
	chord   core.Chord
	mu      sync.Mutex
	wakeR   int
	wakeW   int
	running bool
	exited  chan struct{}
	stopReq atomic.Bool
}

var xGrabMu sync.Mutex

// NewHotkey parses name, then opens a wake pipe. The X display is opened
// later, on the event-loop goroutine, so Start does not touch X from the caller.
func NewHotkey(name string) (core.HotkeySource, error) {
	chord, err := core.ParseHotkey(name)
	if err != nil {
		return nil, err
	}
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return nil, err
	}
	return &x11Hotkey{chord: chord, wakeR: p[0], wakeW: p[1]}, nil
}

// Start grabs the chord and returns after the loop is armed.
func (h *x11Hotkey) Start(fn func()) error {
	if fn == nil {
		return errors.New("nil hotkey callback")
	}
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return errors.New("hotkey already started")
	}
	if h.stopReq.Load() {
		h.mu.Unlock()
		return errors.New("hotkey stopped")
	}
	exited := make(chan struct{})
	h.exited = exited
	h.running = true
	h.mu.Unlock()

	errCh := make(chan error, 1)
	go h.loop(fn, errCh, exited)
	if err := <-errCh; err != nil {
		<-exited
		h.mu.Lock()
		h.running = false
		h.exited = nil
		h.mu.Unlock()
		return err
	}
	return nil
}

// Stop asks the loop to exit and waits until it has. It is safe to call
// more than once, including before Start.
func (h *x11Hotkey) Stop() {
	h.stopReq.Store(true)
	h.mu.Lock()
	w := h.wakeW
	exited := h.exited
	h.mu.Unlock()
	if w >= 0 {
		_, _ = unix.Write(w, []byte{1})
	}
	if exited != nil {
		<-exited
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.wakeR >= 0 {
		_ = unix.Close(h.wakeR)
		h.wakeR = -1
	}
	if h.wakeW >= 0 {
		_ = unix.Close(h.wakeW)
		h.wakeW = -1
	}
}

func (h *x11Hotkey) loop(fn func(), errCh chan<- error, exited chan struct{}) {
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
		signal(errors.New("hotkey loop exited before it was armed"))
	}()

	h.mu.Lock()
	wake := h.wakeR
	h.mu.Unlock()
	if wake < 0 || h.stopReq.Load() {
		signal(errors.New("hotkey stopped"))
		return
	}

	dpy := C.hkOpen()
	if dpy == nil {
		signal(fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY")))
		return
	}
	defer C.hkClose(dpy)

	sym, err := keysymOf(h.chord.Key)
	if err != nil {
		signal(err)
		return
	}
	kc := int(C.csKeycode(dpy, sym))
	if kc == 0 {
		signal(fmt.Errorf("no keycode for %s", h.chord.Key))
		return
	}
	if err := grabChord(dpy, kc, h.chord); err != nil {
		signal(err)
		return
	}
	defer ungrabChord(dpy, kc, h.chord)

	xfd := int(C.csConn(dpy))
	if xfd < 0 {
		signal(errors.New("XConnectionNumber failed"))
		return
	}
	signal(nil)

	for {
		if h.stopReq.Load() {
			return
		}
		for C.csPending(dpy) != 0 {
			if C.csTakeKey(dpy, C.int(kc)) != 0 {
				fn()
			}
		}
		if err := pollX(xfd, wake); err != nil {
			return
		}
	}
}

func grabChord(dpy *C.Display, keycode int, chord core.Chord) error {
	xGrabMu.Lock()
	defer xGrabMu.Unlock()
	C.csHotkeyResetErr()
	C.csHotkeyInstallHandler()
	defer C.csHotkeyRestoreHandler()
	C.csSelectKeys(dpy)
	masks := chordMasks(chord)
	for _, m := range masks {
		C.csGrabKey(dpy, C.int(keycode), m)
	}
	C.csSync(dpy)
	if code := int(C.csHotkeyErr()); code != 0 {
		for _, m := range masks {
			C.csUngrabKey(dpy, C.int(keycode), m)
		}
		C.csSync(dpy)
		return fmt.Errorf("XGrabKey failed (X error %d)", code)
	}
	return nil
}

func ungrabChord(dpy *C.Display, keycode int, chord core.Chord) {
	xGrabMu.Lock()
	defer xGrabMu.Unlock()
	for _, m := range chordMasks(chord) {
		C.csUngrabKey(dpy, C.int(keycode), m)
	}
	C.csSync(dpy)
}

func chordMasks(chord core.Chord) []C.uint {
	base := x11Mods(chord)
	return []C.uint{
		C.uint(base),
		C.uint(base | x11Lock),
		C.uint(base | x11Mod2),
		C.uint(base | x11Lock | x11Mod2),
	}
}

// pollX waits until the X connection or the wake pipe is readable.
// A wake, a hangup, or a closed wake fd ends the loop.
func pollX(xfd, wake int) error {
	fds := []unix.PollFd{
		{Fd: int32(xfd), Events: unix.POLLIN},
		{Fd: int32(wake), Events: unix.POLLIN},
	}
	for {
		n, err := unix.Poll(fds, 200)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return err
		}
		if fds[1].Revents != 0 {
			return errors.New("hotkey wake")
		}
		if fds[0].Revents != 0 || n == 0 {
			return nil
		}
	}
}

func keysymOf(key string) (C.ulong, error) {
	switch key {
	case "escape":
		return 0xff1b, nil
	case "return":
		return 0xff0d, nil
	case "tab":
		return 0xff09, nil
	case "backspace":
		return 0xff08, nil
	case "delete":
		return 0xffff, nil
	case "insert":
		return 0xff63, nil
	case "home":
		return 0xff50, nil
	case "end":
		return 0xff57, nil
	case "pageup":
		return 0xff55, nil
	case "pagedown":
		return 0xff56, nil
	case "left":
		return 0xff51, nil
	case "up":
		return 0xff52, nil
	case "right":
		return 0xff53, nil
	case "down":
		return 0xff54, nil
	case "space":
		return 0x20, nil
	}
	if len(key) >= 2 && key[0] == 'f' {
		n, err := strconv.Atoi(key[1:])
		if err == nil && n >= 1 && n <= 24 {
			return C.ulong(0xffbe + n - 1), nil
		}
	}
	if len(key) == 1 {
		return C.ulong(key[0]), nil
	}
	return 0, fmt.Errorf("no keysym for %q", key)
}

// x11TapKey presses and releases name on a second Display connection.
// Tests call it only after refusing shared displays.
func x11TapKey(name string) error {
	chord, err := core.ParseHotkey(name)
	if err != nil {
		return err
	}
	sym, err := keysymOf(chord.Key)
	if err != nil {
		return err
	}
	dpy := C.hkOpen()
	if dpy == nil {
		return fmt.Errorf("XOpenDisplay failed (DISPLAY=%q)", os.Getenv("DISPLAY"))
	}
	defer C.hkClose(dpy)
	if C.hkXTest(dpy) == 0 {
		return errors.New("XTest extension is not available")
	}
	// Root must be focused or the passive grab on the root window does not
	// activate. This helper is only used by the private-display test.
	xGrabMu.Lock()
	C.csHotkeyResetErr()
	C.csHotkeyInstallHandler()
	focusRC := C.hkFocusRoot(dpy)
	focusErr := int(C.csHotkeyErr())
	C.csHotkeyRestoreHandler()
	xGrabMu.Unlock()
	if focusRC != 0 || focusErr != 0 {
		return fmt.Errorf("XSetInputFocus failed (rc %d, X error %d)", focusRC, focusErr)
	}
	kc := C.csKeycode(dpy, sym)
	if kc == 0 {
		return fmt.Errorf("no keycode for %s", chord.Key)
	}
	if C.csTapKey(dpy, C.uint(kc)) != 0 {
		return errors.New("XTestFakeKeyEvent failed")
	}
	return nil
}
