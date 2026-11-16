package engine

import (
	"encoding/json"
	"errors"
	"strconv"

	"github.com/click-spirit/click-spirit/core"
)

// Pick event kinds queued by StartPick.
const (
	EventPick         = "pick"
	EventPickCanceled = "pick_canceled"
	EventPickError    = "pick_error"
)

// ErrPickerUnsupported is returned when no on-screen picker is installed.
var ErrPickerUnsupported = errors.New("on-screen picking is not supported on this platform")

// ErrPickBusy is returned when a pick is already in progress.
var ErrPickBusy = errors.New("a screen pick is already in progress")

// SetPicker installs the on-screen picker. Nil removes it.
func (e *Engine) SetPicker(p core.ScreenPicker) {
	e.mu.Lock()
	e.picker = p
	e.mu.Unlock()
}

// HasPicker reports whether on-screen picking is available.
func (e *Engine) HasPicker() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.picker != nil
}

type pickPayload struct {
	Kind    string `json:"kind"`
	Loop    int    `json:"loop"`
	Step    int    `json:"step"`
	Mode    string `json:"mode"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	R       int    `json:"r"`
	Message string `json:"message"`
}

// StartPick opens the picker on its own goroutine and returns at once. The
// outcome is queued as one event: "pick" (x, y, r), "pick_canceled", or
// "pick_error" (message). It refuses to start while a script runs.
func (e *Engine) StartPick(mode core.PickMode) error {
	if mode != core.PickPoint && mode != core.PickCircle {
		return errors.New("pick mode must be point or circle")
	}
	e.mu.Lock()
	p := e.picker
	switch {
	case p == nil:
		e.mu.Unlock()
		return ErrPickerUnsupported
	case e.picking:
		e.mu.Unlock()
		return ErrPickBusy
	case e.starting || (e.runner != nil && e.runner.Running()):
		e.mu.Unlock()
		return core.ErrAlreadyRunning
	}
	e.picking = true
	e.mu.Unlock()

	go func() {
		res, err := p.Pick(mode)
		out := pickPayload{Mode: string(mode), Step: -1}
		switch {
		case errors.Is(err, core.ErrPickCanceled):
			out.Kind = EventPickCanceled
			out.Message = "已取消"
		case err != nil:
			out.Kind = EventPickError
			out.Message = err.Error()
		default:
			out.Kind = EventPick
			out.X, out.Y, out.R = res.X, res.Y, res.R
			if mode == core.PickCircle {
				out.Message = "圓心 " + itoa(res.X) + ", " + itoa(res.Y) + " · r=" + itoa(res.R)
			} else {
				out.Message = "座標 " + itoa(res.X) + ", " + itoa(res.Y)
			}
		}
		b, _ := json.Marshal(out)
		e.mu.Lock()
		e.picking = false
		e.pushLocked(string(b))
		e.mu.Unlock()
	}()
	return nil
}

func itoa(n int) string { return strconv.Itoa(n) }
