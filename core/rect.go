package core

// Rect is a screen rectangle in clicker coordinates. W and H <= 0 is empty.
type Rect struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Contains reports whether (x, y) lies inside r (right and bottom edges
// excluded).
func (r Rect) Contains(x, y int) bool {
	return r.W > 0 && r.H > 0 && x >= r.X && y >= r.Y && x < r.X+r.W && y < r.Y+r.H
}

// AnyContains reports whether any rect contains (x, y).
func AnyContains(rects []Rect, x, y int) bool {
	for _, r := range rects {
		if r.Contains(x, y) {
			return true
		}
	}
	return false
}
