package core

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrStopped is returned when a run is canceled with Stop or a parent context.
// The error also wraps the context error (context.Canceled or DeadlineExceeded).
var ErrStopped = errors.New("stopped")

// ErrAlreadyRunning is returned when Start or Run is called while a run is active.
var ErrAlreadyRunning = errors.New("already running")

// EventKind identifies a runner progress event.
type EventKind string

const (
	EventStarted  EventKind = "started"
	EventStep     EventKind = "step"
	EventLoopDone EventKind = "loopDone"
	EventFinished EventKind = "finished"
	EventStopped  EventKind = "stopped"
	EventError    EventKind = "error"
)

// Event is one progress notification.
// StepIndex is -1 when the event is not tied to a step.
type Event struct {
	Kind      EventKind
	Loop      int
	StepIndex int
	Step      Step
	Err       error
}

// Clicker performs pointer actions.
type Clicker interface {
	MoveTo(x, y int) error
	Click(b Button, count int, holdMs int) error
}

// CursorLocator is implemented by clickers that can read the pointer position.
type CursorLocator interface {
	CursorPos() (x, y int, err error)
}

type flight struct {
	clicker Clicker
	sleep   func(context.Context, time.Duration) error
	onEvent func(Event)
}

type runState struct {
	done chan struct{}
	err  error
}

// Runner executes a script.
//
// Set Clicker, Sleep, and OnEvent before Start or Run. Sleep defaults to a
// context-aware timer. Per-step IntervalMs of 0 uses Script.DefaultIntervalMs.
// The delay is applied after each click. LoopIntervalMs is applied between
// loops, not after the final loop. Repeat of 0 loops until Stop or cancel.
type Runner struct {
	Clicker Clicker
	Sleep   func(ctx context.Context, d time.Duration) error
	OnEvent func(Event)

	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	state   *runState
}

// Run executes the script on the calling goroutine.
// A stop returns an error that wraps both ErrStopped and the context error.
func (r *Runner) Run(ctx context.Context, script *Script) error {
	cp, err := normalizeScript(script)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	f, err := r.begin(cancel)
	if err != nil {
		cancel()
		return err
	}
	err = r.loop(runCtx, cp, f)
	r.finish(nil, err)
	cancel()
	return err
}

// Start executes the script on a new goroutine.
// It returns ErrAlreadyRunning if a run is already active.
func (r *Runner) Start(ctx context.Context, script *Script) error {
	cp, err := normalizeScript(script)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	st := &runState{done: make(chan struct{})}
	f, err := r.begin(cancel)
	if err != nil {
		cancel()
		return err
	}
	r.mu.Lock()
	r.state = st
	r.mu.Unlock()

	go func() {
		runErr := r.loop(runCtx, cp, f)
		r.finish(st, runErr)
		cancel()
		close(st.done)
	}()
	return nil
}

// Stop cancels the active run. It is safe to call more than once.
func (r *Runner) Stop() {
	r.mu.Lock()
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Running reports whether a run is active.
func (r *Runner) Running() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// Wait blocks until the run started by Start finishes and returns its error.
// It returns nil if Start has not been called.
func (r *Runner) Wait() error {
	r.mu.Lock()
	st := r.state
	r.mu.Unlock()
	if st == nil {
		return nil
	}
	<-st.done
	r.mu.Lock()
	err := st.err
	r.mu.Unlock()
	return err
}

func (r *Runner) begin(cancel context.CancelFunc) (flight, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		return flight{}, ErrAlreadyRunning
	}
	if r.Clicker == nil {
		return flight{}, errors.New("nil clicker")
	}
	f := flight{clicker: r.Clicker, sleep: r.Sleep, onEvent: r.OnEvent}
	if f.sleep == nil {
		f.sleep = defaultSleep
	}
	r.running = true
	r.cancel = cancel
	return f, nil
}

func (r *Runner) finish(st *runState, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st != nil {
		st.err = err
	}
	r.running = false
	r.cancel = nil
}

func normalizeScript(script *Script) (*Script, error) {
	if script == nil {
		return nil, errors.New("nil script")
	}
	if err := script.Validate(); err != nil {
		return nil, err
	}
	cp := *script
	cp.Steps = append([]Step(nil), script.Steps...)
	return &cp, nil
}

func (r *Runner) loop(ctx context.Context, s *Script, f flight) error {
	f.emit(Event{Kind: EventStarted, Loop: 0, StepIndex: -1})
	if err := f.wait(ctx, s.StartDelayMs); err != nil {
		return f.endWait(ctx, err, 0, -1, Step{})
	}
	for loop := 0; s.Repeat == 0 || loop < s.Repeat; loop++ {
		if err := ctx.Err(); err != nil {
			return f.halt(ctx, loop)
		}
		for i, step := range s.Steps {
			if err := ctx.Err(); err != nil {
				return f.halt(ctx, loop)
			}
			f.emit(Event{Kind: EventStep, Loop: loop, StepIndex: i, Step: step})
			if err := f.clicker.MoveTo(step.X, step.Y); err != nil {
				f.emit(Event{Kind: EventError, Loop: loop, StepIndex: i, Step: step, Err: err})
				return err
			}
			if err := f.clicker.Click(step.Button, step.EffectiveClicks(), step.HoldMs); err != nil {
				f.emit(Event{Kind: EventError, Loop: loop, StepIndex: i, Step: step, Err: err})
				return err
			}
			if err := f.wait(ctx, step.EffectiveIntervalMs(s.DefaultIntervalMs)); err != nil {
				return f.endWait(ctx, err, loop, i, step)
			}
		}
		f.emit(Event{Kind: EventLoopDone, Loop: loop, StepIndex: -1})
		if s.Repeat == 0 || loop+1 < s.Repeat {
			if err := f.wait(ctx, s.LoopIntervalMs); err != nil {
				return f.endWait(ctx, err, loop, -1, Step{})
			}
		}
	}
	f.emit(Event{Kind: EventFinished, Loop: 0, StepIndex: -1})
	return nil
}

func (f flight) emit(ev Event) {
	if f.onEvent != nil {
		f.onEvent(ev)
	}
}

func (f flight) wait(ctx context.Context, ms int) error {
	if ms <= 0 {
		return nil
	}
	return f.sleep(ctx, time.Duration(ms)*time.Millisecond)
}

func (f flight) endWait(ctx context.Context, err error, loop, index int, step Step) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return f.halt(ctx, loop)
	}
	f.emit(Event{Kind: EventError, Loop: loop, StepIndex: index, Step: step, Err: err})
	return err
}

func (f flight) halt(ctx context.Context, loop int) error {
	f.emit(Event{Kind: EventStopped, Loop: loop, StepIndex: -1})
	return stopError(ctx)
}

func stopError(ctx context.Context) error {
	cause := context.Canceled
	if ctx != nil && ctx.Err() != nil {
		cause = ctx.Err()
	}
	return fmt.Errorf("%w: %w", ErrStopped, cause)
}

func defaultSleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
