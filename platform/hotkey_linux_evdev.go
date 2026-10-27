//go:build linux

package platform

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/unix"

	"github.com/click-spirit/click-spirit/core"
)

// evdevHotkey listens for a chord on keyboards that expose F1–F12.
// It is the Linux hook when X11 is not available (no DISPLAY, cgo off,
// or -tags nox11). Reading the nodes requires membership in the input group.
// The callback runs on the event-loop goroutine and must not call Stop.
type evdevHotkey struct {
	chord   core.Chord
	mu      sync.Mutex
	wakeR   int
	wakeW   int
	running bool
	exited  chan struct{}
	stopReq atomic.Bool
}

func newEvdevHotkey(chord core.Chord) (core.HotkeySource, error) {
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		return nil, err
	}
	return &evdevHotkey{chord: chord, wakeR: p[0], wakeW: p[1]}, nil
}

func (h *evdevHotkey) Start(fn func()) error {
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
		h.closePipe()
		return err
	}
	return nil
}

func (h *evdevHotkey) Stop() {
	h.stopReq.Store(true)
	h.wake()
	h.mu.Lock()
	exited := h.exited
	h.mu.Unlock()
	if exited != nil {
		<-exited
	}
	h.closePipe()
}

func (h *evdevHotkey) wake() {
	h.mu.Lock()
	w := h.wakeW
	h.mu.Unlock()
	if w >= 0 {
		_, _ = unix.Write(w, []byte{1})
	}
}

func (h *evdevHotkey) closePipe() {
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

func (h *evdevHotkey) loop(fn func(), errCh chan<- error, exited chan struct{}) {
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
	if _, ok := evdevKeyCode(h.chord.Key); !ok {
		signal(fmt.Errorf("no evdev keycode for %s", h.chord.Key))
		return
	}
	devs, err := openEvdevDevices(keyboardKeyBits, false)
	if err != nil {
		signal(err)
		return
	}
	defer closeEvdevDevs(devs)
	if len(devs) == 0 {
		signal(fmt.Errorf("no keyboard with KEY_F1..KEY_F12 in /dev/input: %w", ErrUnsupported))
		return
	}
	signal(nil)

	fds := make([]int, len(devs))
	for i, d := range devs {
		fds[i] = d.fd
	}
	var mods modifierState
	for {
		if h.stopReq.Load() {
			return
		}
		for _, d := range devs {
			evs, rerr := d.readEvents()
			if rerr != nil {
				return
			}
			for _, ev := range evs {
				if ev.Type != evKey {
					continue
				}
				if chordHit(h.chord, ev.Code, ev.Value, mods) {
					fn()
				}
				mods.apply(ev.Code, ev.Value)
			}
		}
		if err := pollFDs(fds, wake); err != nil {
			return
		}
	}
}
