//go:build linux && cgo && !nox11

package platform

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

func TestPickFromLayer(t *testing.T) {
	p := pickFromLayer(core.PickPoint, [8]int{0, 0, 123, 45, 0, 0, 1600, 1600})
	if p.X != 123 || p.Y != 45 {
		t.Fatalf("point %+v", p)
	}
	// HiDPI-style capture (2x pixels) on a monitor at x=-1920.
	c := pickFromLayer(core.PickCircle, [8]int{400, 400, 460, 480, -1920, 0, 3840, 1920})
	if c.X != -1720 || c.Y != 200 || c.R != 50 {
		t.Fatalf("circle %+v", c)
	}
}

func startPick(t *testing.T, mode core.PickMode) chan struct {
	r   core.PickResult
	err error
} {
	ch := make(chan struct {
		r   core.PickResult
		err error
	}, 1)
	p := &x11Picker{}
	go func() {
		r, err := p.Pick(mode)
		ch <- struct {
			r   core.PickResult
			err error
		}{r, err}
	}()
	deadline := time.Now().Add(5 * time.Second)
	for !pickerMapped() {
		if time.Now().After(deadline) {
			t.Fatal("picker window never mapped")
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond)
	return ch
}

// TestX11ScreenPicker runs only with CLICK_SPIRIT_X11_TEST=1 under xvfb-run.
func TestX11ScreenPicker(t *testing.T) {
	if os.Getenv("CLICK_SPIRIT_X11_TEST") != "1" {
		t.Skip("set CLICK_SPIRIT_X11_TEST=1 under xvfb-run")
	}
	disp := os.Getenv("DISPLAY")
	if disp == "" || forbiddenSharedDisplay(disp) {
		t.Fatalf("refusing display %q; run under xvfb-run", disp)
	}
	wait := func(ch chan struct {
		r   core.PickResult
		err error
	}) (core.PickResult, error) {
		select {
		case v := <-ch:
			return v.r, v.err
		case <-time.After(5 * time.Second):
			t.Fatal("picker did not return")
		}
		return core.PickResult{}, nil
	}

	ch := startPick(t, core.PickPoint)
	pickFakeMotion(123, 45)
	time.Sleep(50 * time.Millisecond)
	pickFakeButton(1, true)
	pickFakeButton(1, false)
	r, err := wait(ch)
	if err != nil || r.X != 123 || r.Y != 45 {
		t.Fatalf("point pick = %+v, %v", r, err)
	}

	ch = startPick(t, core.PickCircle)
	pickFakeMotion(200, 200)
	pickFakeButton(1, true)
	pickFakeMotion(215, 220)
	pickFakeMotion(230, 240)
	time.Sleep(50 * time.Millisecond)
	pickFakeButton(1, false)
	r, err = wait(ch)
	if err != nil || r.X != 200 || r.Y != 200 || r.R != 50 {
		t.Fatalf("circle pick = %+v, %v", r, err)
	}

	ch = startPick(t, core.PickPoint)
	pickFakeEscape()
	if _, err := wait(ch); !errors.Is(err, core.ErrPickCanceled) {
		t.Fatalf("escape err = %v", err)
	}
	if pickerMapped() {
		t.Fatal("picker window still mapped after cancel")
	}
}
