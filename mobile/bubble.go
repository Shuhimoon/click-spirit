package mobile

import (
	"sync"

	"github.com/click-spirit/click-spirit/core"
	"github.com/click-spirit/click-spirit/internal/engine"
)

// rectBubble mirrors the Kotlin floating bubble's screen rectangle so the Go
// runner skips steps that would tap it. The bubble itself (drawing, taps,
// actions) lives in Kotlin.
type rectBubble struct {
	mu      sync.Mutex
	r       core.Rect
	visible bool
}

func (b *rectBubble) SetVisible(bool)          {}
func (b *rectBubble) SetState(bool, bool)      {}
func (b *rectBubble) SetOnAction(func(string)) {}

func (b *rectBubble) Rect() (core.Rect, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.r, b.visible
}

var bubble = &rectBubble{}

// SetBubbleRect tells the runner where the floating bubble is, in screen
// pixels. visible=false (or w/h <= 0) means taps are never skipped.
func SetBubbleRect(x, y, w, h int, visible bool) {
	bubble.mu.Lock()
	bubble.r = core.Rect{X: x, Y: y, W: w, H: h}
	bubble.visible = visible && w > 0 && h > 0
	bubble.mu.Unlock()
}

// ClicksToScript turns recorded taps (JSON array of {x,y,button,at_ms,hold_ms})
// into script JSON: one point step per tap, gaps become intervals.
func ClicksToScript(clicksJSON, name string) (string, error) {
	return engine.ClicksToScript(clicksJSON, name)
}
