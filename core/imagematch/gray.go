// Package imagematch finds a grayscale template inside a screen image.
// Matching is zero-mean normalized cross-correlation (ZNCC). It tolerates a
// brightness and contrast shift. It does not tolerate scale or rotation.
package imagematch

import (
	"bytes"
	"image"
	"image/png"
)

// Gray is a row-major grayscale image. Pixels are 0..255.
type Gray struct {
	W, H int
	Pix  []float32
}

// FromImage converts img to grayscale with Rec. 601 weights.
// The result origin is the top-left of img.Bounds.
func FromImage(img image.Image) *Gray {
	if img == nil {
		return &Gray{}
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return &Gray{}
	}
	g := &Gray{W: w, H: h, Pix: make([]float32, w*h)}
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, gg, bb, _ := img.At(x, y).RGBA()
			g.Pix[i] = float32(luma8(r, gg, bb))
			i++
		}
	}
	return g
}

func luma8(r, g, b uint32) uint32 {
	// RGBA() is 16-bit. Shift to 8-bit, then apply Rec. 601.
	r8 := r >> 8
	g8 := g >> 8
	b8 := b >> 8
	return (19595*r8 + 38470*g8 + 7471*b8 + 32768) >> 16
}

// DecodePNG decodes a PNG blob.
func DecodePNG(data []byte) (image.Image, error) {
	return png.Decode(bytes.NewReader(data))
}

func downsample(g *Gray) *Gray {
	w, h := g.W/2, g.H/2
	if w < 1 || h < 1 {
		return g
	}
	out := &Gray{W: w, H: h, Pix: make([]float32, w*h)}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum float32
			var n float32
			for dy := 0; dy < 2; dy++ {
				for dx := 0; dx < 2; dx++ {
					sum += g.Pix[(y*2+dy)*g.W+(x*2+dx)]
					n++
				}
			}
			out.Pix[y*w+x] = sum / n
		}
	}
	return out
}
