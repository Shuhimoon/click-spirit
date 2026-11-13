package core

import (
	"context"
	"image"
	"math"
	"math/rand/v2"
	"testing"
	"time"
)

// moveLog remembers every MoveTo so click events can be checked against the
// position the clicker actually received.
type moveLog struct {
	moves [][2]int
}

func (m *moveLog) MoveTo(x, y int) error {
	m.moves = append(m.moves, [2]int{x, y})
	return nil
}

func (m *moveLog) Click(Button, int, int) error { return nil }

func noSleep(context.Context, time.Duration) error { return nil }

func TestRunnerEmitsClickEventsInOrderWithPositions(t *testing.T) {
	src := rand.New(rand.NewPCG(7, 11))
	clk := &moveLog{}
	var clicks []Event
	r := &Runner{
		Clicker: clk,
		Sleep:   noSleep,
		Rand:    src.Float64,
		OnEvent: func(ev Event) {
			if ev.Kind == EventClick {
				clicks = append(clicks, ev)
			}
		},
	}
	const cx, cy, radius = 400, 300, 50
	script := &Script{
		Version: 1,
		Repeat:  2,
		Steps: []Step{
			{X: 10, Y: 20, Button: ButtonLeft, Repeat: 2},
			{Button: ButtonLeft, Repeat: 200, Target: &Target{Kind: TargetCircle, CX: cx, CY: cy, R: radius}},
			{X: 700, Y: 80, Button: ButtonRight},
		},
	}
	if err := r.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}

	type want struct{ loop, step, rep, repeats int }
	var order []want
	for loop := 0; loop < 2; loop++ {
		for i, n := range []int{2, 200, 1} {
			for rep := 0; rep < n; rep++ {
				order = append(order, want{loop, i, rep, n})
			}
		}
	}
	if len(clicks) != len(order) {
		t.Fatalf("got %d click events, want %d", len(clicks), len(order))
	}
	if len(clk.moves) != len(clicks) {
		t.Fatalf("got %d moves for %d clicks", len(clk.moves), len(clicks))
	}
	for k, ev := range clicks {
		w := order[k]
		if ev.Loop != w.loop || ev.StepIndex != w.step || ev.Repetition != w.rep || ev.Repeats != w.repeats {
			t.Fatalf("event %d = loop %d step %d rep %d/%d, want %+v", k, ev.Loop, ev.StepIndex, ev.Repetition, ev.Repeats, w)
		}
		if got := [2]int{ev.Step.X, ev.Step.Y}; got != clk.moves[k] {
			t.Fatalf("event %d at %v, clicker moved to %v", k, got, clk.moves[k])
		}
		switch ev.StepIndex {
		case 0:
			if ev.Step.X != 10 || ev.Step.Y != 20 {
				t.Fatalf("point step clicked at (%d,%d)", ev.Step.X, ev.Step.Y)
			}
		case 1:
			// Integer rounding can push a point at most ~0.71 px past the edge.
			d := math.Hypot(float64(ev.Step.X-cx), float64(ev.Step.Y-cy))
			if d > radius+0.75 {
				t.Fatalf("circle click (%d,%d) is %.2f px from center, radius %d", ev.Step.X, ev.Step.Y, d, radius)
			}
		case 2:
			if ev.Step.X != 700 || ev.Step.Y != 80 || ev.Step.Button != ButtonRight {
				t.Fatalf("step 3 clicked %+v", ev.Step)
			}
		}
	}
}

func TestRunnerClickEventFollowsStepEvent(t *testing.T) {
	var kinds []EventKind
	r := &Runner{
		Clicker: &moveLog{},
		Sleep:   noSleep,
		OnEvent: func(ev Event) { kinds = append(kinds, ev.Kind) },
	}
	script := &Script{Version: 1, Repeat: 1, Steps: []Step{{X: 1, Y: 2, Button: ButtonLeft}}}
	if err := r.Run(context.Background(), script); err != nil {
		t.Fatal(err)
	}
	want := []EventKind{EventStarted, EventStep, EventClick, EventLoopDone, EventFinished}
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kinds = %v, want %v", kinds, want)
		}
	}
}

func TestRunnerImageClickEventUsesMatchCenter(t *testing.T) {
	screen, templ := scene(t)
	b64 := pngBase64(t, templ)
	miss := image.NewRGBA(screen.Bounds())

	for _, tc := range []struct {
		name   string
		img    image.Image
		clicks int
	}{
		{"hit", screen, 1},
		{"skip", miss, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var clicks []Event
			r := &Runner{
				Clicker:  &moveLog{},
				Sleep:    noSleep,
				Capturer: stillCap{img: tc.img},
				OnEvent: func(ev Event) {
					if ev.Kind == EventClick {
						clicks = append(clicks, ev)
					}
				},
			}
			if err := r.Run(context.Background(), imageScript(b64, OnNotFoundSkip, 0, 0)); err != nil {
				t.Fatal(err)
			}
			if len(clicks) != tc.clicks {
				t.Fatalf("got %d click events, want %d", len(clicks), tc.clicks)
			}
			if tc.clicks == 1 && (clicks[0].Step.X != 10 || clicks[0].Step.Y != 8) {
				t.Fatalf("image click at (%d,%d), want (10,8)", clicks[0].Step.X, clicks[0].Step.Y)
			}
		})
	}
}
