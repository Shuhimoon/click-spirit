//go:build windows

package platform

import (
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/click-spirit/click-spirit/core"
)

const (
	wmHotkey    = 0x0312
	wmQuit      = 0x0012
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

var (
	hkUser32               = windows.NewLazySystemDLL("user32.dll")
	procRegisterHotKey     = hkUser32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = hkUser32.NewProc("UnregisterHotKey")
	procGetMessageW        = hkUser32.NewProc("GetMessageW")
	procPeekMessageW       = hkUser32.NewProc("PeekMessageW")
	procPostThreadMessageW = hkUser32.NewProc("PostThreadMessageW")

	hotkeyIDs atomic.Uint32
)

// winHotkey registers one chord and waits on a message loop pinned to an OS thread.
// The callback runs on that thread and must not call Stop.
type winHotkey struct {
	chord   core.Chord
	id      uintptr
	mu      sync.Mutex
	tid     uint32
	running bool
	exited  chan struct{}
	stopReq atomic.Bool
}

// NewHotkey parses name. RegisterHotKey runs later, on the message-loop thread.
func NewHotkey(name string) (core.HotkeySource, error) {
	chord, err := core.ParseHotkey(name)
	if err != nil {
		return nil, err
	}
	return &winHotkey{chord: chord, id: uintptr(hotkeyIDs.Add(1))}, nil
}

// Start registers the chord and returns after the message queue is armed.
func (h *winHotkey) Start(fn func()) error {
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

// Stop posts WM_QUIT to the message thread and waits until UnregisterHotKey has run.
// It is safe to call more than once, including before Start.
func (h *winHotkey) Stop() {
	h.stopReq.Store(true)
	h.mu.Lock()
	tid := h.tid
	exited := h.exited
	h.mu.Unlock()
	if tid != 0 {
		_, _, _ = procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
	}
	if exited != nil {
		<-exited
	}
}

func (h *winHotkey) loop(fn func(), errCh chan<- error, exited chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
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

	if h.stopReq.Load() {
		signal(errors.New("hotkey stopped"))
		return
	}

	// PeekMessage creates this thread's queue so a later WM_QUIT can be posted.
	var msg winMSG
	_, _, _ = procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)

	tid := windows.GetCurrentThreadId()
	h.mu.Lock()
	h.tid = tid
	h.mu.Unlock()
	if h.stopReq.Load() {
		signal(errors.New("hotkey stopped"))
		return
	}

	vk, err := vkOf(h.chord.Key)
	if err != nil {
		signal(err)
		return
	}
	r, _, callErr := procRegisterHotKey.Call(0, h.id, winMods(h.chord), vk)
	if r == 0 {
		signal(fmt.Errorf("RegisterHotKey: %v", callErr))
		return
	}
	defer procUnregisterHotKey.Call(0, h.id)

	signal(nil)
	for {
		if h.stopReq.Load() {
			return
		}
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(ret) == -1 || ret == 0 {
			return
		}
		if msg.Message == wmHotkey && msg.WParam == h.id {
			fn()
		}
	}
}

func winMods(c core.Chord) uintptr {
	m := uintptr(modNoRepeat)
	if c.Alt {
		m |= modAlt
	}
	if c.Ctrl {
		m |= modControl
	}
	if c.Shift {
		m |= modShift
	}
	if c.Super {
		m |= modWin
	}
	return m
}

func vkOf(key string) (uintptr, error) {
	switch key {
	case "escape":
		return 0x1B, nil
	case "return":
		return 0x0D, nil
	case "tab":
		return 0x09, nil
	case "backspace":
		return 0x08, nil
	case "delete":
		return 0x2E, nil
	case "insert":
		return 0x2D, nil
	case "home":
		return 0x24, nil
	case "end":
		return 0x23, nil
	case "pageup":
		return 0x21, nil
	case "pagedown":
		return 0x22, nil
	case "left":
		return 0x25, nil
	case "up":
		return 0x26, nil
	case "right":
		return 0x27, nil
	case "down":
		return 0x28, nil
	case "space":
		return 0x20, nil
	}
	if len(key) >= 2 && key[0] == 'f' {
		n, err := strconv.Atoi(key[1:])
		if err == nil && n >= 1 && n <= 24 {
			return uintptr(0x70 + n - 1), nil
		}
	}
	if len(key) == 1 {
		c := key[0]
		if c >= 'a' && c <= 'z' {
			return uintptr(0x41 + c - 'a'), nil
		}
		if c >= '0' && c <= '9' {
			return uintptr(c), nil
		}
	}
	return 0, fmt.Errorf("no virtual key for %q", key)
}
