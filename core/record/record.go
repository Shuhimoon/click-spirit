// Package record turns timestamped pointer events into a click script.
// Linux hooks implement InputSource. Callers can also push events with
// Recorder.Add or with engine.StartRecordingWith.
package record

import (
	"errors"
	"sync"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// Kind is a recorded pointer event.
type Kind string

const (
	KindMove  Kind = "move"
	KindDown  Kind = "down"
	KindUp    Kind = "up"
	KindClick Kind = "click"
)

// Event is one pointer sample. Time is the moment it happened.
type Event struct {
	Kind   Kind
	Button core.Button
	X      int
	Y      int
	Time   time.Time
}

// InputSource delivers live pointer events until Stop.
// Platform hooks implement this later. Tests use a fake.
type InputSource interface {
	Start(fn func(Event)) error
	Stop()
}

// Recorder stores events in arrival order.
type Recorder struct {
	mu     sync.Mutex
	events []Event
	src    InputSource
	on     bool
}

// New returns an empty recorder.
func New() *Recorder {
	return &Recorder{}
}

// Add appends an event. A zero time is replaced with time.Now.
// Add is safe to call while a source is running.
func (r *Recorder) Add(ev Event) {
	if ev.Time.IsZero() {
		ev.Time = time.Now()
	}
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
}

// Start clears previous events and records from src until Stop.
func (r *Recorder) Start(src InputSource) error {
	if src == nil {
		return errors.New("nil input source")
	}
	r.mu.Lock()
	if r.on {
		r.mu.Unlock()
		return errors.New("already recording")
	}
	r.events = nil
	r.src = src
	r.on = true
	r.mu.Unlock()
	if err := src.Start(func(ev Event) { r.Add(ev) }); err != nil {
		r.mu.Lock()
		r.on = false
		r.src = nil
		r.mu.Unlock()
		return err
	}
	return nil
}

// Stop ends an active source. It is safe to call more than once.
func (r *Recorder) Stop() {
	r.mu.Lock()
	src := r.src
	r.src = nil
	r.on = false
	r.mu.Unlock()
	if src != nil {
		src.Stop()
	}
}

// Active reports whether Start has been called and Stop has not.
func (r *Recorder) Active() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.on
}

// Events returns a copy of the recorded events.
func (r *Recorder) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}
