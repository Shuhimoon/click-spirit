package core

import (
	"math"
	"math/rand/v2"
	"testing"
)

func TestPointInDiscBasic(t *testing.T) {
	// theta = 0, rho = R → the rightmost point.
	x, y := PointInDisc(10, 20, 5, 0, 1)
	if x != 15 || y != 20 {
		t.Fatalf("got %d,%d", x, y)
	}
	// Degenerate disc is the center. The samples are ignored.
	x, y = PointInDisc(3, 4, 0, 0.3, 0.8)
	if x != 3 || y != 4 {
		t.Fatalf("center %d,%d", x, y)
	}
	// A sample that lands left of the origin is clamped to x = 0.
	x, y = PointInDisc(0, 0, 10, 0.5, 1)
	if x != 0 || y != 0 {
		t.Fatalf("clamp %d,%d", x, y)
	}
	// Half-pixel rounding: rho = 2.5 straight up.
	x, y = PointInDisc(10, 10, 5, 0.25, 0.25)
	if x != 10 || y != 13 {
		t.Fatalf("round %d,%d", x, y)
	}
}

func TestPointInDiscUniform(t *testing.T) {
	const (
		n       = 200_000
		rings   = 10
		sectors = 12
		cx      = 800
		cy      = 700
		radius  = 300
	)
	rng := rand.New(rand.NewPCG(1, 2))
	ringCount := make([]int, rings)
	sectorCount := make([]int, sectors)
	var sumR float64
	for i := 0; i < n; i++ {
		x, y := PointInDisc(cx, cy, radius, rng.Float64(), rng.Float64())
		if x < 0 || y < 0 {
			t.Fatalf("negative pixel %d,%d", x, y)
		}
		dx := float64(x - cx)
		dy := float64(y - cy)
		dist := math.Hypot(dx, dy)
		if dist > float64(radius)+0.75 {
			t.Fatalf("point %d,%d is outside the disc (dist %.3f)", x, y, dist)
		}
		sumR += dist
		ringCount[ringIndex(dist, float64(radius), rings)]++
		sectorCount[sectorIndex(dx, dy, sectors)]++
	}
	mean := sumR / float64(n)
	wantMean := 2 * float64(radius) / 3
	if math.Abs(mean-wantMean) > 2 {
		t.Fatalf("mean radius %.3f, want about %.3f", mean, wantMean)
	}
	chiR := chiSquare(ringCount)
	chiS := chiSquare(sectorCount)
	// df = bins-1. A correct sampler lands near df. 80 is far above the
	// p=0.001 critical value and still rejects a badly biased sampler.
	if chiR > 80 {
		t.Fatalf("ring chi-square %.2f counts %v", chiR, ringCount)
	}
	if chiS > 80 {
		t.Fatalf("sector chi-square %.2f counts %v", chiS, sectorCount)
	}
}

// TestNaiveRadiusFailsRings documents why rho = R*sqrt(u) is required.
// rho = R*u is uniform in radius, so equal-area rings are not equally likely.
func TestNaiveRadiusFailsRings(t *testing.T) {
	const (
		n      = 200_000
		rings  = 10
		cx     = 800
		cy     = 700
		radius = 300
	)
	rng := rand.New(rand.NewPCG(3, 4))
	counts := make([]int, rings)
	for i := 0; i < n; i++ {
		x, y := naivePoint(cx, cy, radius, rng.Float64(), rng.Float64())
		dx := float64(x - cx)
		dy := float64(y - cy)
		counts[ringIndex(math.Hypot(dx, dy), float64(radius), rings)]++
	}
	chi := chiSquare(counts)
	if chi < 1000 {
		t.Fatalf("naive sampler chi-square %.2f, want a large value; counts %v", chi, counts)
	}
}

// naivePoint samples rho = R*u. That is uniform along a radius and therefore
// denser near the center when the disc is split into equal-area rings.
func naivePoint(cx, cy, r int, u1, u2 float64) (int, int) {
	theta := 2 * math.Pi * u1
	rho := float64(r) * u2
	x := int(math.Round(float64(cx) + rho*math.Cos(theta)))
	y := int(math.Round(float64(cy) + rho*math.Sin(theta)))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// ringIndex maps a distance into one of k equal-area rings.
// Ring i covers radii (R*sqrt(i/k), R*sqrt((i+1)/k)].
func ringIndex(dist, radius float64, k int) int {
	if radius <= 0 {
		return 0
	}
	norm := dist / radius
	if norm < 0 {
		norm = 0
	}
	idx := int(math.Floor(norm*norm*float64(k) + 1e-9))
	if idx < 0 {
		return 0
	}
	if idx >= k {
		return k - 1
	}
	return idx
}

func sectorIndex(dx, dy float64, k int) int {
	theta := math.Atan2(dy, dx)
	if theta < 0 {
		theta += 2 * math.Pi
	}
	idx := int(theta / (2 * math.Pi) * float64(k))
	if idx < 0 {
		return 0
	}
	if idx >= k {
		return k - 1
	}
	return idx
}

func chiSquare(counts []int) float64 {
	n := 0
	for _, c := range counts {
		n += c
	}
	if len(counts) == 0 || n == 0 {
		return 0
	}
	expected := float64(n) / float64(len(counts))
	var sum float64
	for _, c := range counts {
		d := float64(c) - expected
		sum += d * d / expected
	}
	return sum
}
