package record

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/click-spirit/click-spirit/core"
)

// Options controls how events become a script.
//
// Speed scales holds and intervals. 2 plays twice as fast. Values <= 0 mean 1.
// MinIntervalMs is a floor on the delay after each step.
// DoubleClickMs merges consecutive clicks of the same button that land within
// MoveTolerance pixels. 0 means 300. A negative value disables the merge.
// MoveTolerance 0 means 3 pixels. The merged step uses clicks=N (at most 3)
// and the first click's hold. The gap inside a double-click is not preserved;
// the script model plays the N clicks back to back.
type Options struct {
	Speed         float64
	MinIntervalMs int
	DoubleClickMs int
	MoveTolerance int
	Name          string
}

type action struct {
	button core.Button
	x, y   int
	start  time.Time
	end    time.Time
}

// ToScript converts events into a one-loop script.
// Moves are ignored. A down followed by an up of the same button becomes one
// click whose hold_ms is the time the button was down. The interval after a
// step is the time from that click's release to the next click's press,
// divided by Speed, and then raised to MinIntervalMs.
func ToScript(events []Event, opt Options) (*core.Script, error) {
	opt = normalize(opt)
	actions := clicksFrom(events)
	if len(actions) == 0 {
		return nil, errors.New("recording has no clicks")
	}
	steps := merge(actions, opt)
	name := opt.Name
	if name == "" {
		name = "錄製"
	}
	def := opt.MinIntervalMs
	if def < 0 {
		def = 0
	}
	return &core.Script{
		Version:           1,
		Name:              name,
		Steps:             steps,
		Repeat:            1,
		DefaultIntervalMs: def,
		LoopIntervalMs:    0,
		StartDelayMs:      0,
	}, nil
}

func normalize(opt Options) Options {
	if opt.Speed <= 0 {
		opt.Speed = 1
	}
	if opt.DoubleClickMs == 0 {
		opt.DoubleClickMs = 300
	}
	if opt.MoveTolerance == 0 {
		opt.MoveTolerance = 3
	}
	if opt.MoveTolerance < 0 {
		opt.MoveTolerance = 0
	}
	if opt.MinIntervalMs < 0 {
		opt.MinIntervalMs = 0
	}
	return opt
}

func clicksFrom(events []Event) []action {
	sorted := append([]Event(nil), events...)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Time.Before(sorted[j].Time)
	})
	open := map[core.Button][]Event{}
	var actions []action
	for _, ev := range sorted {
		switch ev.Kind {
		case KindDown:
			open[ev.Button] = append(open[ev.Button], ev)
		case KindUp:
			stack := open[ev.Button]
			if len(stack) == 0 {
				continue
			}
			down := stack[len(stack)-1]
			open[ev.Button] = stack[:len(stack)-1]
			end := ev.Time
			if end.Before(down.Time) {
				end = down.Time
			}
			actions = append(actions, action{
				button: down.Button,
				x:      down.X,
				y:      down.Y,
				start:  down.Time,
				end:    end,
			})
		case KindClick:
			actions = append(actions, action{
				button: ev.Button,
				x:      ev.X,
				y:      ev.Y,
				start:  ev.Time,
				end:    ev.Time,
			})
		}
	}
	sort.SliceStable(actions, func(i, j int) bool {
		return actions[i].start.Before(actions[j].start)
	})
	return actions
}

func merge(actions []action, opt Options) []core.Step {
	steps := make([]core.Step, 0, len(actions))
	for i := 0; i < len(actions); {
		group := []action{actions[i]}
		j := i + 1
		if opt.DoubleClickMs > 0 {
			for j < len(actions) && len(group) < 3 {
				prev := group[len(group)-1]
				next := actions[j]
				if next.button != prev.button || !near(prev, next, opt.MoveTolerance) {
					break
				}
				gap := next.start.Sub(prev.start)
				if gap > time.Duration(opt.DoubleClickMs)*time.Millisecond {
					break
				}
				group = append(group, next)
				j++
			}
		}
		last := group[len(group)-1]
		hold := lastHold(group[0], opt.Speed)
		interval := 0
		if j < len(actions) {
			interval = scaleMs(actions[j].start.Sub(last.end), opt.Speed)
		}
		if interval < opt.MinIntervalMs {
			interval = opt.MinIntervalMs
		}
		// IntervalMs 0 means "use the script default". When the computed delay
		// is 0 and the default is also 0, leaving it at 0 waits zero time.
		steps = append(steps, core.Step{
			X:          clamp0(group[0].x),
			Y:          clamp0(group[0].y),
			Button:     group[0].button,
			Clicks:     len(group),
			IntervalMs: interval,
			HoldMs:     hold,
		})
		i = j
	}
	return steps
}

func lastHold(a action, speed float64) int {
	return scaleMs(a.end.Sub(a.start), speed)
}

func scaleMs(d time.Duration, speed float64) int {
	if d <= 0 {
		return 0
	}
	ms := math.Round(float64(d) / float64(time.Millisecond) / speed)
	if ms < 0 {
		return 0
	}
	if ms > float64(math.MaxInt) {
		return math.MaxInt
	}
	return int(ms)
}

func near(a, b action, tol int) bool {
	dx := a.x - b.x
	if dx < 0 {
		dx = -dx
	}
	dy := a.y - b.y
	if dy < 0 {
		dy = -dy
	}
	return dx <= tol && dy <= tol
}

func clamp0(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
