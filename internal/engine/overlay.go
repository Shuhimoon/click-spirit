package engine

import "github.com/click-spirit/click-spirit/core"

// SetOverlay installs the run-time click overlay. Nil removes it.
func (e *Engine) SetOverlay(o core.ClickOverlay) {
	e.mu.Lock()
	e.overlay = o
	e.mu.Unlock()
}

// SetShowClicks turns the run-time click overlay on or off. It is on by
// default. Click events are queued either way; only the overlay is skipped.
func (e *Engine) SetShowClicks(on bool) {
	e.mu.Lock()
	e.hideClicks = !on
	e.mu.Unlock()
}

// ShowClicks reports whether the run-time click overlay is enabled.
func (e *Engine) ShowClicks() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return !e.hideClicks
}

// HasOverlay reports whether an overlay is installed.
func (e *Engine) HasOverlay() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.overlay != nil
}
