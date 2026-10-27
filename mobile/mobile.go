// Package mobile is the gomobile-bind API used by the Android app.
//
// Exported functions and interfaces use only gomobile-supported types
// (string, int, bool, error, and interfaces of those). A Kotlin
// GestureDispatcher performs the taps. MoveTo remembers the point and Click
// calls Tap.
package mobile

import (
	"errors"
	"sync"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/internal/engine"
)

// GestureDispatcher is implemented on Android by the accessibility service.
// Tap performs count gestures at (x, y). durationMs is how long each gesture is held.
// Return false when the gesture cannot be dispatched.
type GestureDispatcher interface {
	Tap(x, y int, durationMs int, count int) bool
}

// EventListener receives runner progress. step is -1 when the event is not a step.
type EventListener interface {
	OnEvent(kind string, loop int, step int, message string)
}

var (
	mu       sync.Mutex
	disp     GestureDispatcher
	listener EventListener
	clicker  *dispatcherClicker
	eng      *engine.Engine
)

// SetDispatcher installs the platform gesture sink. It may be called again
// to replace the sink while the process is alive.
func SetDispatcher(d GestureDispatcher) {
	mu.Lock()
	defer mu.Unlock()
	disp = d
	if clicker != nil {
		clicker.set(d)
	}
}

// SetListener installs the progress callback. A nil listener disables it.
func SetListener(l EventListener) {
	mu.Lock()
	listener = l
	mu.Unlock()
}

// ValidateScript parses and validates script JSON.
func ValidateScript(jsonStr string) error {
	s, err := core.Unmarshal([]byte(jsonStr))
	if err != nil {
		return err
	}
	return s.Validate()
}

// SampleScript returns the built-in sample as JSON.
func SampleScript() string {
	b, err := core.NewSample().Marshal()
	if err != nil {
		return ""
	}
	return string(b)
}

// StartScript validates jsonStr and runs it. It returns an error if the
// dispatcher is missing or a script is already running.
func StartScript(jsonStr string) error {
	mu.Lock()
	e, err := ensureLocked()
	mu.Unlock()
	if err != nil {
		return err
	}
	return e.Start(jsonStr)
}

// Stop ends the active script.
func Stop() {
	mu.Lock()
	e := eng
	mu.Unlock()
	if e != nil {
		e.Stop()
	}
}

// IsRunning reports whether a script is running.
func IsRunning() bool {
	mu.Lock()
	e := eng
	mu.Unlock()
	if e == nil {
		return false
	}
	return e.Running()
}

func ensureLocked() (*engine.Engine, error) {
	if disp == nil {
		return nil, errors.New("gesture dispatcher not set")
	}
	if clicker == nil {
		clicker = &dispatcherClicker{}
	}
	clicker.set(disp)
	if eng == nil {
		eng = engine.New(clicker, nil)
		eng.SetHook(forwardEvent)
	}
	return eng, nil
}

func forwardEvent(ev core.Event) {
	msg := engine.Message(ev)
	mu.Lock()
	l := listener
	mu.Unlock()
	if l != nil {
		l.OnEvent(string(ev.Kind), ev.Loop, ev.StepIndex, msg)
	}
}

// dispatcherClicker adapts GestureDispatcher to core.Clicker.
// MoveTo only records the position. Click calls Tap. Mouse buttons are not
// represented by Android accessibility gestures.
type dispatcherClicker struct {
	mu   sync.Mutex
	d    GestureDispatcher
	x, y int
}

func (c *dispatcherClicker) set(d GestureDispatcher) {
	c.mu.Lock()
	c.d = d
	c.mu.Unlock()
}

func (c *dispatcherClicker) MoveTo(x, y int) error {
	c.mu.Lock()
	c.x, c.y = x, y
	c.mu.Unlock()
	return nil
}

func (c *dispatcherClicker) Click(b core.Button, count, holdMs int) error {
	if count <= 0 {
		count = 1
	}
	c.mu.Lock()
	d := c.d
	x, y := c.x, c.y
	c.mu.Unlock()
	if d == nil {
		return errors.New("gesture dispatcher not set")
	}
	if !d.Tap(x, y, holdMs, count) {
		return errors.New("gesture dispatch failed")
	}
	return nil
}
