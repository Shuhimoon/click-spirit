package record

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// TimedClick is one detected click of a screen recording (錄影轉腳本):
// where it landed, which button, when it started relative to the start of
// the recording (AtMs, the video timecode) and how long it was held.
type TimedClick struct {
	X      int         `json:"x"`
	Y      int         `json:"y"`
	Button core.Button `json:"button"`
	AtMs   int64       `json:"at_ms"`
	HoldMs int         `json:"hold_ms"`
}

// ClicksFromEvents pairs down/up (and click) events into clicks with
// timecodes relative to start, sorted by time. Presses inside any exclude
// rect (the floating bubble) are dropped. A zero start uses the first event.
func ClicksFromEvents(events []Event, start time.Time, exclude []core.Rect) []TimedClick {
	actions := clicksFrom(events)
	if len(actions) == 0 {
		return nil
	}
	if start.IsZero() {
		start = actions[0].start
	}
	out := make([]TimedClick, 0, len(actions))
	for _, a := range actions {
		if core.AnyContains(exclude, a.x, a.y) {
			continue
		}
		at := a.start.Sub(start).Milliseconds()
		if at < 0 {
			at = 0
		}
		out = append(out, TimedClick{
			X:      a.x,
			Y:      a.y,
			Button: a.button,
			AtMs:   at,
			HoldMs: int(a.end.Sub(a.start).Milliseconds()),
		})
	}
	return out
}

// ScriptFromClicks turns reviewed clicks into a script: every click is one
// point step, in time order; the gap from one click's release to the next
// click's press becomes that step's interval (the last step gets
// lastIntervalMs). Clicks inside exclude are dropped. Recorded scripts start
// with the default countdown.
func ScriptFromClicks(clicks []TimedClick, name string, exclude []core.Rect, lastIntervalMs int) (*core.Script, error) {
	kept := make([]TimedClick, 0, len(clicks))
	for _, c := range clicks {
		if core.AnyContains(exclude, c.X, c.Y) {
			continue
		}
		if c.Button == "" {
			c.Button = core.ButtonLeft
		}
		if !c.Button.Valid() {
			return nil, fmt.Errorf("click at %d ms: invalid button %q", c.AtMs, c.Button)
		}
		kept = append(kept, c)
	}
	if len(kept) == 0 {
		return nil, errors.New("no clicks to convert")
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].AtMs < kept[j].AtMs })
	steps := make([]core.Step, len(kept))
	for i, c := range kept {
		interval := lastIntervalMs
		if i+1 < len(kept) {
			interval = int(kept[i+1].AtMs - c.AtMs - int64(clamp0(c.HoldMs)))
		}
		if interval < 1 {
			// 0 would mean "use the script default"; keep back-to-back clicks
			// back to back.
			interval = 1
		}
		steps[i] = core.Step{
			X:          clamp0(c.X),
			Y:          clamp0(c.Y),
			Button:     c.Button,
			Clicks:     1,
			IntervalMs: interval,
			HoldMs:     clamp0(c.HoldMs),
		}
	}
	if name == "" {
		name = "錄影腳本"
	}
	return &core.Script{
		Version:           1,
		Name:              name,
		Steps:             steps,
		Repeat:            1,
		DefaultIntervalMs: 100,
		StartDelaySec:     core.StartDelay(core.DefaultStartDelaySec),
	}, nil
}

// FilterExcluded drops press/release pairs that start inside exclude. It is
// used live while recording so presses on the bubble never reach a script.
type FilterExcluded struct {
	Exclude    func(x, y int) bool
	suppressed map[core.Button]bool
}

// Keep reports whether ev should be recorded.
func (f *FilterExcluded) Keep(ev Event) bool {
	if f.suppressed == nil {
		f.suppressed = map[core.Button]bool{}
	}
	switch ev.Kind {
	case KindDown:
		if f.Exclude != nil && f.Exclude(ev.X, ev.Y) {
			f.suppressed[ev.Button] = true
			return false
		}
	case KindUp:
		if f.suppressed[ev.Button] {
			delete(f.suppressed, ev.Button)
			return false
		}
	case KindClick:
		if f.Exclude != nil && f.Exclude(ev.X, ev.Y) {
			return false
		}
	}
	return true
}
