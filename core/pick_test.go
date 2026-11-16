package core

import "testing"

func TestScreenGeometryToScreen(t *testing.T) {
	cases := []struct {
		name   string
		g      ScreenGeometry
		px, py float64
		x, y   int
	}{
		{"identity", ScreenGeometry{}, 123, 45, 123, 45},
		{"fractional pixel floors", ScreenGeometry{}, 10.9, 20.2, 10, 20},
		{"secondary monitor left of primary", ScreenGeometry{OriginX: -1920, OriginY: 0, Scale: 1}, 100, 50, -1820, 50},
		{"monitor above", ScreenGeometry{OriginX: 0, OriginY: -1080, Scale: 1}, 5, 1079, 5, -1},
		{"retina 2x", ScreenGeometry{Scale: 2}, 401, 300, 200, 150},
		{"150% with offset", ScreenGeometry{OriginX: 1920, OriginY: 0, Scale: 1.5}, 300, 150, 2120, 100},
		{"bad scale counts as 1", ScreenGeometry{OriginX: 7, Scale: -3}, 3, 4, 10, 4},
	}
	for _, c := range cases {
		x, y := c.g.ToScreen(c.px, c.py)
		if x != c.x || y != c.y {
			t.Errorf("%s: ToScreen(%v,%v) = %d,%d want %d,%d", c.name, c.px, c.py, x, y, c.x, c.y)
		}
	}
}

func TestScreenGeometryRoundTrip(t *testing.T) {
	for _, g := range []ScreenGeometry{{}, {OriginX: -1920, OriginY: -200, Scale: 1}, {OriginX: 30, OriginY: 40, Scale: 2}, {Scale: 1.25}} {
		for _, p := range [][2]int{{0, 0}, {17, 933}, {-1900, -150}, {2559, 1439}} {
			px, py := g.ToPixel(p[0], p[1])
			x, y := g.ToScreen(px, py)
			if x != p[0] || y != p[1] {
				t.Errorf("%+v: %v -> (%v,%v) -> %d,%d", g, p, px, py, x, y)
			}
		}
	}
	if r := (ScreenGeometry{Scale: 2}).RadiusToScreen(81); r != 41 {
		t.Errorf("RadiusToScreen = %d want 41", r)
	}
}

func TestCircleFromDrag(t *testing.T) {
	cases := []struct{ px, py, rx, ry, cx, cy, r int }{
		{200, 200, 230, 240, 200, 200, 50},
		{10, 10, 10, 10, 10, 10, 0},
		{100, 100, 90, 100, 100, 100, 10},
		{0, 0, 1, 1, 0, 0, 1}, // 1.414 rounds to 1
		{0, 0, 2, 2, 0, 0, 3}, // 2.828 rounds to 3
		{50, 60, -10, 60, 50, 60, 60},
	}
	for _, c := range cases {
		cx, cy, r := CircleFromDrag(c.px, c.py, c.rx, c.ry)
		if cx != c.cx || cy != c.cy || r != c.r {
			t.Errorf("CircleFromDrag(%d,%d,%d,%d) = %d,%d,%d want %d,%d,%d", c.px, c.py, c.rx, c.ry, cx, cy, r, c.cx, c.cy, c.r)
		}
	}
}
