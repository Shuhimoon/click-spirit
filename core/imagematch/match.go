package imagematch

import (
	"encoding/json"
	"errors"
	"image"
	"math"
	"sort"
)

// Result is one template placement. X and Y are the top-left of the match.
// Score is the zero-mean normalized cross-correlation in [-1, 1].
type Result struct {
	X, Y  int
	Score float64
}

// Hit is the JSON shape returned to FFI and mobile.
// X and Y are the center of the match (the default click point) when Found
// is true, and 0 when it is not. Score is the best ZNCC even when Found is false.
type Hit struct {
	Found bool    `json:"found"`
	X     int     `json:"x"`
	Y     int     `json:"y"`
	Score float64 `json:"score"`
	Error string  `json:"error"`
}

// JSON encodes the hit. Marshaling errors become a found:false payload.
func (h Hit) JSON() string {
	b, err := json.Marshal(h)
	if err != nil {
		return `{"found":false,"x":0,"y":0,"score":0,"error":"json"}`
	}
	return string(b)
}

// DefaultThreshold is the score treated as a match when the caller passes <= 0.
const DefaultThreshold = 0.9

// MatchImages searches screen for template and reports the click point.
func MatchImages(screen, template image.Image, threshold float64) Hit {
	if screen == nil || template == nil {
		return Hit{Error: "nil image"}
	}
	sg := FromImage(screen)
	tg := FromImage(template)
	best, err := FindBest(sg, tg, image.Rectangle{})
	if err != nil {
		return Hit{Error: err.Error()}
	}
	return hitFrom(best, tg, threshold)
}

// MatchPNG decodes both PNG blobs and searches.
func MatchPNG(screenPNG, templatePNG []byte, threshold float64) Hit {
	screen, err := DecodePNG(screenPNG)
	if err != nil {
		return Hit{Error: "screen png: " + err.Error()}
	}
	templ, err := DecodePNG(templatePNG)
	if err != nil {
		return Hit{Error: "template png: " + err.Error()}
	}
	return MatchImages(screen, templ, threshold)
}

func hitFrom(best Result, templ *Gray, threshold float64) Hit {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	h := Hit{Score: best.Score}
	if best.Score < threshold || templ == nil {
		return h
	}
	h.Found = true
	h.X = best.X + templ.W/2
	h.Y = best.Y + templ.H/2
	if h.X < 0 {
		h.X = 0
	}
	if h.Y < 0 {
		h.Y = 0
	}
	return h
}

// FindBest returns the highest-scoring placement of templ inside region.
// An empty region searches the whole screen. The template must fit.
// Large inputs are searched coarse-to-fine and then refined at full resolution.
// A weak coarse peak falls back to a full scan so the result stays exact.
func FindBest(screen, templ *Gray, region image.Rectangle) (Result, error) {
	if err := checkPair(screen, templ); err != nil {
		return Result{}, err
	}
	region = normalizeRegion(screen, region)
	if region.Dx() < templ.W || region.Dy() < templ.H {
		return Result{}, errors.New("template does not fit in the search region")
	}
	return findBest(screen, templ, region, 0)
}

// FindAll returns placements whose score is at least threshold, after
// non-maximum suppression. Suppression keeps the stronger peak when two
// placements are within half the template's larger side.
func FindAll(screen, templ *Gray, region image.Rectangle, threshold float64) ([]Result, error) {
	if err := checkPair(screen, templ); err != nil {
		return nil, err
	}
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	region = normalizeRegion(screen, region)
	if region.Dx() < templ.W || region.Dy() < templ.H {
		return nil, errors.New("template does not fit in the search region")
	}
	cands, err := scanAll(screen, templ, region, threshold)
	if err != nil {
		return nil, err
	}
	return suppress(cands, templ), nil
}

func findBest(screen, templ *Gray, region image.Rectangle, depth int) (Result, error) {
	area := int64(region.Dx()) * int64(region.Dy()) * int64(templ.W) * int64(templ.H)
	usePyramid := depth < 2 && templ.W >= 16 && templ.H >= 16 && screen.W >= 48 && screen.H >= 48 && area > 4_000_000
	if !usePyramid {
		return scanBest(screen, templ, region)
	}
	s2 := downsample(screen)
	t2 := downsample(templ)
	if t2.W < 8 || t2.H < 8 || s2.W < t2.W || s2.H < t2.H {
		return scanBest(screen, templ, region)
	}
	coarseRegion := image.Rect(region.Min.X/2, region.Min.Y/2, (region.Max.X+1)/2, (region.Max.Y+1)/2)
	coarse, err := findBest(s2, t2, coarseRegion, depth+1)
	if err != nil {
		return scanBest(screen, templ, region)
	}
	if coarse.Score < 0.5 {
		return scanBest(screen, templ, region)
	}
	const margin = 16
	refine := image.Rect(
		coarse.X*2-margin,
		coarse.Y*2-margin,
		coarse.X*2+margin+templ.W,
		coarse.Y*2+margin+templ.H,
	).Intersect(region)
	if refine.Dx() < templ.W || refine.Dy() < templ.H {
		return scanBest(screen, templ, region)
	}
	best, err := scanBest(screen, templ, refine)
	if err != nil {
		return Result{}, err
	}
	// The coarse peak can sit on the wrong object. If refinement is clearly
	// worse than a full-resolution search would be, pay for the full scan.
	if best.Score+0.02 < coarse.Score {
		return scanBest(screen, templ, region)
	}
	return best, nil
}

func checkPair(screen, templ *Gray) error {
	if screen == nil || templ == nil || screen.W <= 0 || screen.H <= 0 || len(screen.Pix) < screen.W*screen.H {
		return errors.New("empty screen")
	}
	if templ.W <= 0 || templ.H <= 0 || len(templ.Pix) < templ.W*templ.H {
		return errors.New("empty template")
	}
	if templ.W > screen.W || templ.H > screen.H {
		return errors.New("template is larger than the screen")
	}
	return nil
}

func normalizeRegion(screen *Gray, region image.Rectangle) image.Rectangle {
	full := image.Rect(0, 0, screen.W, screen.H)
	if region.Empty() {
		return full
	}
	return region.Intersect(full)
}

type prepared struct {
	t0    []float64
	sumT2 float64
	integ *integral
	tw    int
	th    int
}

func prepare(screen, templ *Gray) (prepared, error) {
	n := float64(len(templ.Pix))
	var sum float64
	for _, v := range templ.Pix {
		sum += float64(v)
	}
	mean := sum / n
	t0 := make([]float64, len(templ.Pix))
	var sumT2 float64
	for i, v := range templ.Pix {
		d := float64(v) - mean
		t0[i] = d
		sumT2 += d * d
	}
	if sumT2 < 1e-6 {
		return prepared{}, errors.New("template has no contrast")
	}
	return prepared{t0: t0, sumT2: sumT2, integ: buildIntegral(screen), tw: templ.W, th: templ.H}, nil
}

func scanBest(screen, templ *Gray, region image.Rectangle) (Result, error) {
	p, err := prepare(screen, templ)
	if err != nil {
		return Result{}, err
	}
	x1 := region.Max.X - templ.W + 1
	y1 := region.Max.Y - templ.H + 1
	if region.Min.X >= x1 || region.Min.Y >= y1 {
		return Result{}, errors.New("template does not fit in the search region")
	}
	best := Result{X: region.Min.X, Y: region.Min.Y, Score: -2}
	for y := region.Min.Y; y < y1; y++ {
		for x := region.Min.X; x < x1; x++ {
			score := p.scoreAt(screen, x, y)
			if score > best.Score {
				best.X = x
				best.Y = y
				best.Score = score
			}
		}
	}
	if best.Score < -1 {
		best.Score = -1
	}
	return best, nil
}

func scanAll(screen, templ *Gray, region image.Rectangle, threshold float64) ([]Result, error) {
	p, err := prepare(screen, templ)
	if err != nil {
		return nil, err
	}
	x1 := region.Max.X - templ.W + 1
	y1 := region.Max.Y - templ.H + 1
	var out []Result
	for y := region.Min.Y; y < y1; y++ {
		for x := region.Min.X; x < x1; x++ {
			score := p.scoreAt(screen, x, y)
			if score >= threshold {
				out = append(out, Result{X: x, Y: y, Score: score})
			}
		}
	}
	return out, nil
}

func (p prepared) scoreAt(screen *Gray, x, y int) float64 {
	n := float64(p.tw * p.th)
	s, s2 := p.integ.rect(x, y, p.tw, p.th)
	varI := s2 - s*s/n
	if varI < 1e-6 {
		return 0
	}
	cross := 0.0
	for j := 0; j < p.th; j++ {
		row := screen.Pix[(y+j)*screen.W+x : (y+j)*screen.W+x+p.tw]
		base := j * p.tw
		for i := 0; i < p.tw; i++ {
			cross += float64(row[i]) * p.t0[base+i]
		}
	}
	score := cross / math.Sqrt(varI*p.sumT2)
	if score > 1 {
		return 1
	}
	if score < -1 {
		return -1
	}
	return score
}

func suppress(cands []Result, templ *Gray) []Result {
	if len(cands) == 0 {
		return nil
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].Score == cands[j].Score {
			if cands[i].Y == cands[j].Y {
				return cands[i].X < cands[j].X
			}
			return cands[i].Y < cands[j].Y
		}
		return cands[i].Score > cands[j].Score
	})
	radius := templ.W
	if templ.H > radius {
		radius = templ.H
	}
	radius /= 2
	if radius < 1 {
		radius = 1
	}
	kept := make([]Result, 0, len(cands))
	for _, c := range cands {
		drop := false
		for _, k := range kept {
			dx := c.X - k.X
			if dx < 0 {
				dx = -dx
			}
			dy := c.Y - k.Y
			if dy < 0 {
				dy = -dy
			}
			if dx <= radius && dy <= radius {
				drop = true
				break
			}
		}
		if !drop {
			kept = append(kept, c)
		}
	}
	return kept
}

// integral is a summed-area table for pixel sums and sums of squares.
type integral struct {
	w    int
	sum  []float64
	sum2 []float64
}

func buildIntegral(g *Gray) *integral {
	stride := g.W + 1
	sum := make([]float64, stride*(g.H+1))
	sum2 := make([]float64, stride*(g.H+1))
	for y := 0; y < g.H; y++ {
		var row, row2 float64
		for x := 0; x < g.W; x++ {
			v := float64(g.Pix[y*g.W+x])
			row += v
			row2 += v * v
			above := y * stride
			here := (y+1)*stride + (x + 1)
			sum[here] = sum[above+(x+1)] + row
			sum2[here] = sum2[above+(x+1)] + row2
		}
	}
	return &integral{w: g.W, sum: sum, sum2: sum2}
}

func (in *integral) rect(x, y, w, h int) (float64, float64) {
	stride := in.w + 1
	x2, y2 := x+w, y+h
	s := in.sum[y2*stride+x2] - in.sum[y*stride+x2] - in.sum[y2*stride+x] + in.sum[y*stride+x]
	s2 := in.sum2[y2*stride+x2] - in.sum2[y*stride+x2] - in.sum2[y2*stride+x] + in.sum2[y*stride+x]
	return s, s2
}
