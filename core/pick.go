package core

import (
	"errors"
	"math"
)

// PickMode selects what the on-screen picker returns.
type PickMode string

const (
	// PickPoint returns the screen pixel the user clicked.
	PickPoint PickMode = "point"
	// PickCircle returns a circle: press at the centre, drag out the radius.
	PickCircle PickMode = "circle"
)

// ErrPickCanceled is returned when the user presses Esc (or the picker
// times out) without choosing anything.
var ErrPickCanceled = errors.New("pick canceled")

// PickResult is a point or circle in clicker coordinates: the same physical
// screen pixels that Clicker.MoveTo takes.
type PickResult struct {
	Mode PickMode
	X, Y int // point, or circle centre
	R    int // circle radius; 0 for points
}

// ScreenPicker shows a full-screen layer over a fresh screenshot and lets
// the user pick a point or drag out a circle. Pick blocks until the user
// picks, cancels (ErrPickCanceled), or an error occurs.
type ScreenPicker interface {
	Pick(mode PickMode) (PickResult, error)
}

// ScreenGeometry maps pixels of a screenshot (or of a full-screen picker
// layer showing it) to clicker coordinates.
//
// Origin is the screen coordinate of the screenshot's top-left pixel. It is
// negative when a monitor sits left of or above the primary one (Windows
// virtual screen, macOS displays arranged to the left). Scale is screenshot
// pixels per clicker unit: 1 on X11 and Windows (physical pixels), 2 for a
// Retina capture when the clicker uses points. A Scale <= 0 counts as 1.
type ScreenGeometry struct {
	OriginX, OriginY int
	Scale            float64
}

func (g ScreenGeometry) scale() float64 {
	if g.Scale <= 0 || math.IsNaN(g.Scale) || math.IsInf(g.Scale, 0) {
		return 1
	}
	return g.Scale
}

// ToScreen converts a screenshot pixel to a clicker coordinate.
func (g ScreenGeometry) ToScreen(px, py float64) (int, int) {
	s := g.scale()
	return g.OriginX + int(math.Floor(px/s)), g.OriginY + int(math.Floor(py/s))
}

// ToPixel converts a clicker coordinate to the screenshot pixel at its
// top-left corner.
func (g ScreenGeometry) ToPixel(x, y int) (float64, float64) {
	s := g.scale()
	return float64(x-g.OriginX) * s, float64(y-g.OriginY) * s
}

// RadiusToScreen converts a radius measured in screenshot pixels.
func (g ScreenGeometry) RadiusToScreen(r float64) int {
	return int(math.Round(r / g.scale()))
}

// CircleFromDrag turns a drag into a circle: the press point is the centre
// and the distance to the release point, rounded, is the radius.
func CircleFromDrag(pressX, pressY, releaseX, releaseY int) (cx, cy, r int) {
	d := math.Hypot(float64(releaseX-pressX), float64(releaseY-pressY))
	return pressX, pressY, int(math.Round(d))
}
