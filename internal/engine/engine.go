// Package engine runs a click script and queues progress events.
// FFI and the mobile package share it. The queue is polled so callers do not
// need a cross-thread callback. When the queue is full, the oldest event is dropped.
package engine

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"sync"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// ErrCaptureUnsupported is returned when the clicker cannot capture the screen.
var ErrCaptureUnsupported = errors.New("screen capture is not supported")

const defaultQueue = 256

// Engine runs one script at a time on top of a core.Clicker.
type Engine struct {
	Sleep func(ctx context.Context, d time.Duration) error

	mu       sync.Mutex
	clicker  core.Clicker
	initErr  error
	runner   *core.Runner
	starting bool
	queue    []string
	maxQueue int
	hook     func(core.Event)
}

// New returns an engine. initErr is returned by Start when clicker is nil,
// which is how desktop FFI reports an unsupported platform.
func New(clicker core.Clicker, initErr error) *Engine {
	return &Engine{
		clicker:  clicker,
		initErr:  initErr,
		maxQueue: defaultQueue,
	}
}

// SetHook installs a callback invoked after each event is queued.
// The hook runs without the engine lock.
func (e *Engine) SetHook(fn func(core.Event)) {
	e.mu.Lock()
	e.hook = fn
	e.mu.Unlock()
}

// Start parses jsonStr, validates it, and runs it asynchronously.
func (e *Engine) Start(jsonStr string) error {
	script, err := core.Unmarshal([]byte(jsonStr))
	if err != nil {
		return err
	}
	if err := script.Validate(); err != nil {
		return err
	}

	e.mu.Lock()
	if e.clicker == nil {
		err := e.initErr
		e.mu.Unlock()
		if err == nil {
			err = errors.New("no clicker")
		}
		return err
	}
	if e.starting || (e.runner != nil && e.runner.Running()) {
		e.mu.Unlock()
		return core.ErrAlreadyRunning
	}
	e.starting = true
	r := &core.Runner{
		Clicker: e.clicker,
		Sleep:   e.Sleep,
		OnEvent: e.onRunnerEvent,
	}
	e.runner = r
	e.mu.Unlock()

	err = r.Start(context.Background(), script)
	e.mu.Lock()
	e.starting = false
	e.mu.Unlock()
	return err
}

// Stop requests the active run to end.
func (e *Engine) Stop() {
	e.mu.Lock()
	r := e.runner
	e.mu.Unlock()
	if r != nil {
		r.Stop()
	}
}

// Running reports whether a script is running.
func (e *Engine) Running() bool {
	e.mu.Lock()
	r := e.runner
	e.mu.Unlock()
	if r == nil {
		return false
	}
	return r.Running()
}

// Wait blocks until the active Start finishes.
func (e *Engine) Wait() error {
	e.mu.Lock()
	r := e.runner
	e.mu.Unlock()
	if r == nil {
		return nil
	}
	return r.Wait()
}

// PollEvent returns the next queued event JSON. The bool is false when empty.
func (e *Engine) PollEvent() (string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.queue) == 0 {
		return "", false
	}
	s := e.queue[0]
	e.queue = e.queue[1:]
	if len(e.queue) == 0 {
		e.queue = nil
	}
	return s, true
}

// Capture returns a screenshot when the clicker implements core.ScreenCapturer.
func (e *Engine) Capture() (image.Image, error) {
	e.mu.Lock()
	c := e.clicker
	initErr := e.initErr
	e.mu.Unlock()
	if c == nil {
		if initErr != nil {
			return nil, initErr
		}
		return nil, errors.New("no clicker")
	}
	s, ok := c.(core.ScreenCapturer)
	if !ok || s == nil {
		return nil, ErrCaptureUnsupported
	}
	return s.Capture()
}

// CursorPos reads the pointer when the clicker implements core.CursorLocator.
// ok is false when the position is unavailable.
func (e *Engine) CursorPos() (x, y int, ok bool) {
	e.mu.Lock()
	c := e.clicker
	e.mu.Unlock()
	loc, is := c.(core.CursorLocator)
	if !is || loc == nil {
		return 0, 0, false
	}
	x, y, err := loc.CursorPos()
	if err != nil {
		return 0, 0, false
	}
	return x, y, true
}

func (e *Engine) onRunnerEvent(ev core.Event) {
	payload, err := json.Marshal(payloadOf(ev))
	if err != nil {
		return
	}
	e.mu.Lock()
	e.pushLocked(string(payload))
	hook := e.hook
	e.mu.Unlock()
	if hook != nil {
		hook(ev)
	}
}

func (e *Engine) pushLocked(s string) {
	limit := e.maxQueue
	if limit <= 0 {
		limit = defaultQueue
	}
	e.queue = append(e.queue, s)
	if len(e.queue) > limit {
		kept := make([]string, limit)
		copy(kept, e.queue[len(e.queue)-limit:])
		e.queue = kept
	}
}

type eventPayload struct {
	Kind    string `json:"kind"`
	Loop    int    `json:"loop"`
	Step    int    `json:"step"`
	Message string `json:"message"`
	X       int    `json:"x"`
	Y       int    `json:"y"`
	Button  string `json:"button"`
	Clicks  int    `json:"clicks"`
}

func payloadOf(ev core.Event) eventPayload {
	p := eventPayload{
		Kind:    string(ev.Kind),
		Loop:    ev.Loop,
		Step:    ev.StepIndex,
		Message: Message(ev),
		X:       ev.Step.X,
		Y:       ev.Step.Y,
		Button:  string(ev.Step.Button),
		Clicks:  ev.Step.Clicks,
	}
	if ev.Kind == core.EventStep {
		p.Clicks = ev.Step.EffectiveClicks()
	}
	return p
}

// Message is a short Traditional Chinese description of ev.
func Message(ev core.Event) string {
	switch ev.Kind {
	case core.EventStarted:
		return "開始執行"
	case core.EventStep:
		return sprintfStep(ev.Step)
	case core.EventLoopDone:
		return sprintfLoop(ev.Loop)
	case core.EventFinished:
		return "執行完成"
	case core.EventStopped:
		return "已停止"
	case core.EventError:
		if ev.Err != nil {
			return ev.Err.Error()
		}
		return "錯誤"
	default:
		if ev.Err != nil {
			return ev.Err.Error()
		}
		return string(ev.Kind)
	}
}
