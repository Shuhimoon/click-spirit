package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/click-spirit/click-spirit/core/imagematch"
)

func prepareTemplates(steps []Step) ([]*imagematch.Gray, error) {
	out := make([]*imagematch.Gray, len(steps))
	for i, step := range steps {
		if step.target().KindOrPoint() != TargetImage {
			continue
		}
		g, err := loadTemplate(step.target())
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", i, err)
		}
		out[i] = g
	}
	return out, nil
}

func loadTemplate(t Target) (*imagematch.Gray, error) {
	img, err := decodeTemplate(t)
	if err != nil {
		return nil, err
	}
	g := imagematch.FromImage(img)
	if g.W <= 0 || g.H <= 0 {
		return nil, fmt.Errorf("empty template")
	}
	return g, nil
}

func decodeTemplate(t Target) (image.Image, error) {
	if t.TemplateB64 != "" {
		raw, err := decodeBase64Image(t.TemplateB64)
		if err != nil {
			return nil, fmt.Errorf("template_b64: %w", err)
		}
		img, err := png.Decode(bytes.NewReader(raw))
		if err != nil {
			return nil, fmt.Errorf("template png: %w", err)
		}
		return img, nil
	}
	if t.Template == "" {
		return nil, fmt.Errorf("image target needs template or template_b64")
	}
	f, err := os.Open(t.Template)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("template png: %w", err)
	}
	return img, nil
}

func decodeBase64Image(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ','); i >= 0 && strings.Contains(strings.ToLower(s[:i]), "base64") {
		s = s[i+1:]
	}
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\n', '\r', ' ', '\t':
			return -1
		default:
			return r
		}
	}, s)
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawStdEncoding.DecodeString(s)
}

// locate resolves the pixel to click. skip is true when an image miss is ignored.
func (f flight) locate(ctx context.Context, step Step, templ *imagematch.Gray) (x, y int, skip bool, err error) {
	t := step.target()
	switch t.KindOrPoint() {
	case TargetPoint:
		return step.X, step.Y, false, nil
	case TargetCircle:
		x, y = PointInDisc(t.CX, t.CY, t.R, f.unit(), f.unit())
		return x, y, false, nil
	case TargetImage:
		return f.locateImage(ctx, t, templ)
	default:
		return 0, 0, false, fmt.Errorf("unknown target kind %q", t.Kind)
	}
}

func (f flight) unit() float64 {
	if f.rand != nil {
		return f.rand()
	}
	return 0
}

func (f flight) locateImage(ctx context.Context, t Target, templ *imagematch.Gray) (int, int, bool, error) {
	if f.capturer == nil {
		return 0, 0, false, ErrNoCapturer
	}
	if templ == nil {
		return 0, 0, false, fmt.Errorf("missing template")
	}
	mode := t.NotFoundOrSkip()
	threshold := t.EffectiveThreshold()
	poll := t.PollMs
	if poll <= 0 {
		poll = 100
	}
	var deadline time.Time
	if mode == OnNotFoundWait {
		deadline = f.clock().Add(time.Duration(t.TimeoutMs) * time.Millisecond)
	}
	for {
		if err := ctx.Err(); err != nil {
			return 0, 0, false, err
		}
		img, err := f.capturer.Capture()
		if err != nil {
			return 0, 0, false, err
		}
		screen := imagematch.FromImage(img)
		region := imageRegion(screen, t.Region)
		best, err := imagematch.FindBest(screen, templ, region)
		if err != nil {
			return 0, 0, false, err
		}
		if best.Score >= threshold {
			x := best.X + templ.W/2 + t.OffsetX
			y := best.Y + templ.H/2 + t.OffsetY
			if x < 0 {
				x = 0
			}
			if y < 0 {
				y = 0
			}
			return x, y, false, nil
		}
		switch mode {
		case OnNotFoundSkip:
			return 0, 0, true, nil
		case OnNotFoundStop:
			return 0, 0, false, ErrImageNotFound
		case OnNotFoundWait:
			if t.TimeoutMs <= 0 || !f.clock().Before(deadline) {
				return 0, 0, false, ErrImageNotFound
			}
			if err := f.sleep(ctx, time.Duration(poll)*time.Millisecond); err != nil {
				return 0, 0, false, err
			}
		default:
			return 0, 0, false, fmt.Errorf("invalid on_not_found %q", mode)
		}
	}
}

func (f flight) clock() time.Time {
	if f.now != nil {
		return f.now()
	}
	return time.Now()
}

func imageRegion(screen *imagematch.Gray, reg *Region) image.Rectangle {
	full := image.Rect(0, 0, screen.W, screen.H)
	if reg == nil || reg.W <= 0 || reg.H <= 0 {
		return full
	}
	return image.Rect(reg.X, reg.Y, reg.X+reg.W, reg.Y+reg.H).Intersect(full)
}
