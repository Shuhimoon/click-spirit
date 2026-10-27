package core

import "math"

// PointInDisc returns a pixel chosen uniformly over the disc of radius r
// centered at (cx, cy).
//
// u1 and u2 are independent samples in [0, 1). The polar radius uses a square
// root so the distribution is uniform over area, not over radius:
//
//	theta = 2π u1
//	rho   = R * sqrt(u2)
//
// Sampling rho = R*u instead crowds points near the center. Coordinates are
// rounded to integers and clamped to be >= 0. A radius of 0 returns the center.
func PointInDisc(cx, cy, r int, u1, u2 float64) (int, int) {
	if r < 0 {
		r = 0
	}
	u1 = clampUnit(u1)
	u2 = clampUnit(u2)
	theta := 2 * math.Pi * u1
	rho := float64(r) * math.Sqrt(u2)
	x := roundNonNeg(float64(cx) + rho*math.Cos(theta))
	y := roundNonNeg(float64(cy) + rho*math.Sin(theta))
	return x, y
}

func clampUnit(u float64) float64 {
	if u < 0 || math.IsNaN(u) {
		return 0
	}
	if u > 1 {
		return 1
	}
	return u
}

func roundNonNeg(v float64) int {
	n := int(math.Round(v))
	if n < 0 {
		return 0
	}
	return n
}
