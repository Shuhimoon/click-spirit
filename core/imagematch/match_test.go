package imagematch

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"testing"
)

func TestIntegralRect(t *testing.T) {
	g := &Gray{W: 2, H: 2, Pix: []float32{1, 2, 3, 4}}
	in := buildIntegral(g)
	s, s2 := in.rect(0, 0, 2, 2)
	if s != 10 || s2 != 30 {
		t.Fatalf("full %v %v", s, s2)
	}
	s, _ = in.rect(1, 1, 1, 1)
	if s != 4 {
		t.Fatal(s)
	}
	s, _ = in.rect(0, 1, 2, 1)
	if s != 7 {
		t.Fatal(s)
	}
}

func TestFindBestExactBrightnessAndNoise(t *testing.T) {
	rng := rand.New(rand.NewPCG(7, 8))
	screen := image.NewRGBA(image.Rect(0, 0, 90, 70))
	fillNoise(screen, rng)
	templ := plusPattern(15)
	const px, py = 21, 17
	paste(screen, templ, px, py)

	best, err := FindBest(FromImage(screen), FromImage(templ), image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if best.X != px || best.Y != py {
		t.Fatalf("location %d,%d score %f", best.X, best.Y, best.Score)
	}
	if best.Score < 0.999 {
		t.Fatalf("exact score %f", best.Score)
	}

	bright := image.NewRGBA(screen.Bounds())
	copy(bright.Pix, screen.Pix)
	pasteBright(bright, templ, px, py, 40)
	best, err = FindBest(FromImage(bright), FromImage(templ), image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if best.X != px || best.Y != py || best.Score < 0.99 {
		t.Fatalf("bright %+v", best)
	}

	noisy := cloneRGBA(screen)
	pasteNoisy(noisy, templ, px, py, rng, 4)
	best, err = FindBest(FromImage(noisy), FromImage(templ), image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if best.X != px || best.Y != py || best.Score < 0.9 {
		t.Fatalf("noisy %+v", best)
	}
}

func TestAbsentTemplateBelowThreshold(t *testing.T) {
	rng := rand.New(rand.NewPCG(9, 10))
	screen := image.NewRGBA(image.Rect(0, 0, 80, 60))
	fillNoise(screen, rng)
	paste(screen, plusPattern(15), 10, 8)
	other := checker(15)
	best, err := FindBest(FromImage(screen), FromImage(other), image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if best.Score >= 0.9 {
		t.Fatalf("absent score %f at %d,%d", best.Score, best.X, best.Y)
	}
	hit := MatchImages(screen, other, 0.9)
	if hit.Found || hit.Error != "" || hit.X != 0 || hit.Y != 0 {
		t.Fatalf("%+v", hit)
	}
	if hit.Score >= 0.9 {
		t.Fatal(hit.Score)
	}
}

func TestFindAllAndRegion(t *testing.T) {
	rng := rand.New(rand.NewPCG(11, 12))
	screen := image.NewRGBA(image.Rect(0, 0, 140, 110))
	fillNoise(screen, rng)
	templ := plusPattern(15)
	spots := [][2]int{{5, 5}, {80, 10}, {20, 70}}
	for _, s := range spots {
		paste(screen, templ, s[0], s[1])
	}
	sg, tg := FromImage(screen), FromImage(templ)
	all, err := FindAll(sg, tg, image.Rectangle{}, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("got %d %+v", len(all), all)
	}
	got := map[[2]int]bool{}
	for _, m := range all {
		if m.Score < 0.95 {
			t.Fatal(m.Score)
		}
		got[[2]int{m.X, m.Y}] = true
	}
	for _, s := range spots {
		if !got[s] {
			t.Fatalf("missing %v in %+v", s, all)
		}
	}

	region := image.Rect(0, 0, 40, 100)
	best, err := FindBest(sg, tg, region)
	if err != nil {
		t.Fatal(err)
	}
	if best.X != 5 || best.Y != 5 {
		t.Fatalf("region best %+v", best)
	}
	// The right-hand copy is outside this region.
	if best.X == 80 {
		t.Fatal("region leaked")
	}
	left, err := FindAll(sg, tg, region, 0.95)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 2 {
		t.Fatalf("region matches %+v", left)
	}
}

func TestPyramidFindsLargePaste(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 14))
	screen := image.NewRGBA(image.Rect(0, 0, 220, 180))
	fillNoise(screen, rng)
	templ := plusPattern(32)
	const px, py = 77, 53
	paste(screen, templ, px, py)
	best, err := FindBest(FromImage(screen), FromImage(templ), image.Rectangle{})
	if err != nil {
		t.Fatal(err)
	}
	if best.X != px || best.Y != py || best.Score < 0.99 {
		t.Fatalf("%+v", best)
	}
}

func TestFlatTemplateAndTooBig(t *testing.T) {
	screen := &Gray{W: 4, H: 4, Pix: make([]float32, 16)}
	flat := &Gray{W: 2, H: 2, Pix: []float32{3, 3, 3, 3}}
	if _, err := FindBest(screen, flat, image.Rectangle{}); err == nil {
		t.Fatal("expected flat template error")
	}
	big := &Gray{W: 5, H: 2, Pix: make([]float32, 10)}
	if _, err := FindBest(screen, big, image.Rectangle{}); err == nil {
		t.Fatal("expected size error")
	}
}

func TestMatchPNGRoundTrip(t *testing.T) {
	screen := image.NewRGBA(image.Rect(0, 0, 40, 30))
	fillNoise(screen, rand.New(rand.NewPCG(1, 1)))
	templ := plusPattern(11)
	paste(screen, templ, 8, 6)
	hit := MatchPNG(mustPNG(t, screen), mustPNG(t, templ), 0)
	if !hit.Found || hit.Error != "" {
		t.Fatal(hit)
	}
	// 11/2 = 5, so the center is the top-left plus 5.
	if hit.X != 8+5 || hit.Y != 6+5 {
		t.Fatalf("center %d,%d", hit.X, hit.Y)
	}
	if hit.JSON() == "" || hit.Score < 0.99 {
		t.Fatal(hit.JSON())
	}
}

func TestDownsampleAverage(t *testing.T) {
	g := &Gray{W: 2, H: 2, Pix: []float32{0, 2, 4, 6}}
	d := downsample(g)
	if d.W != 1 || d.H != 1 || d.Pix[0] != 3 {
		t.Fatalf("%+v", d)
	}
}

func BenchmarkFindBest(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	screen := image.NewRGBA(image.Rect(0, 0, 320, 240))
	fillNoise(screen, rng)
	templ := plusPattern(32)
	paste(screen, templ, 123, 87)
	sg := FromImage(screen)
	tg := FromImage(templ)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := FindBest(sg, tg, image.Rectangle{})
		if err != nil || res.X != 123 || res.Y != 87 {
			b.Fatalf("%+v %v", res, err)
		}
	}
}

func fillNoise(img *image.RGBA, rng *rand.Rand) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := uint8(rng.IntN(200))
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
}

func plusPattern(n int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+3] = 255
	}
	mid := n / 2
	thick := n / 6
	if thick < 1 {
		thick = 1
	}
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := x-mid, y-mid
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if dx <= thick || dy <= thick {
				img.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	return img
}

func checker(n int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			v := uint8(0)
			if (x/3+y/3)%2 == 0 {
				v = 255
			}
			img.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func paste(dst, src *image.RGBA, x, y int) {
	pasteOp(dst, src, x, y, 0, nil, 0)
}

func pasteBright(dst, src *image.RGBA, x, y, add int) {
	pasteOp(dst, src, x, y, add, nil, 0)
}

func pasteNoisy(dst, src *image.RGBA, x, y int, rng *rand.Rand, amp int) {
	pasteOp(dst, src, x, y, 0, rng, amp)
}

func pasteOp(dst, src *image.RGBA, x, y, add int, rng *rand.Rand, amp int) {
	b := src.Bounds()
	for yy := 0; yy < b.Dy(); yy++ {
		for xx := 0; xx < b.Dx(); xx++ {
			c := src.RGBAAt(b.Min.X+xx, b.Min.Y+yy)
			c.R = clamp8(int(c.R) + add)
			c.G = clamp8(int(c.G) + add)
			c.B = clamp8(int(c.B) + add)
			if rng != nil && amp > 0 {
				d := rng.IntN(amp*2+1) - amp
				c.R = clamp8(int(c.R) + d)
				c.G = clamp8(int(c.G) + d)
				c.B = clamp8(int(c.B) + d)
			}
			dst.SetRGBA(x+xx, y+yy, c)
		}
	}
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

func cloneRGBA(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	copy(out.Pix, src.Pix)
	return out
}

func mustPNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
