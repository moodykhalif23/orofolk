// Package imageproc resizes/re-encodes images for the DAM rendition pipeline.
// The default implementation is pure Go (no CGO), so it builds into the static
// binaries unchanged. WebP/AVIF *output* needs a libvips-backed processor
// (govips, a CGO build variant) implementing the same Processor interface — the
// pure-Go processor decodes WebP input but emits JPEG/PNG only.
package imageproc

import (
	"bytes"
	"context"
	"fmt"
	"image"
	_ "image/gif" // register GIF decoder (input only)
	"image/jpeg"
	"image/png"
	"io"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register WebP decoder (input only)
)

// Preset describes a target rendition.
type Preset struct {
	Name    string
	Width   int
	Height  int
	Fit     string // cover | contain | fill | inside
	Format  string // jpeg | png (webp/avif fall back to jpeg here)
	Quality int    // JPEG quality 1-100
}

// Processor produces a transformed image from a source reader.
type Processor interface {
	// Transform returns the encoded bytes, output dimensions, and the actual
	// output format/extension used (which may differ from p.Format if a format
	// this processor can't encode was requested).
	Transform(ctx context.Context, src io.Reader, p Preset) (out []byte, w, h int, format string, err error)
}

// GoProcessor is the pure-Go implementation.
type GoProcessor struct{}

func (GoProcessor) Transform(_ context.Context, src io.Reader, p Preset) ([]byte, int, int, string, error) {
	img, _, err := image.Decode(src)
	if err != nil {
		return nil, 0, 0, "", fmt.Errorf("decode: %w", err)
	}
	dst := resize(img, p)
	b := dst.Bounds()

	format := normalizeFormat(p.Format)
	var buf bytes.Buffer
	switch format {
	case "png":
		if err := png.Encode(&buf, dst); err != nil {
			return nil, 0, 0, "", err
		}
	default: // jpeg
		q := p.Quality
		if q <= 0 || q > 100 {
			q = 82
		}
		if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: q}); err != nil {
			return nil, 0, 0, "", err
		}
		format = "jpeg"
	}
	return buf.Bytes(), b.Dx(), b.Dy(), format, nil
}

// normalizeFormat maps requested formats to what the pure-Go encoder supports.
func normalizeFormat(f string) string {
	switch f {
	case "png":
		return "png"
	default: // jpeg, webp, avif, "" → jpeg
		return "jpeg"
	}
}

// resize scales src into the preset's box according to the fit mode and returns
// the new image. Missing width/height are derived from the source aspect ratio.
func resize(src image.Image, p Preset) image.Image {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 {
		return src
	}
	tw, th := targetBox(sw, sh, p)
	if tw <= 0 || th <= 0 {
		return src
	}

	switch p.Fit {
	case "cover":
		// Scale to fill the box, center-crop the overflow.
		scale := maxf(float64(tw)/float64(sw), float64(th)/float64(sh))
		rw, rh := int(float64(sw)*scale+0.5), int(float64(sh)*scale+0.5)
		scaled := scaleTo(src, rw, rh)
		ox := (rw - tw) / 2
		oy := (rh - th) / 2
		out := image.NewRGBA(image.Rect(0, 0, tw, th))
		draw.Copy(out, image.Point{}, scaled, image.Rect(ox, oy, ox+tw, oy+th), draw.Src, nil)
		return out
	case "fill":
		// Stretch to exact box, ignoring aspect ratio.
		return scaleTo(src, tw, th)
	default: // contain | inside → fit inside the box, preserve aspect ratio
		scale := minf(float64(tw)/float64(sw), float64(th)/float64(sh))
		if p.Fit == "inside" && scale > 1 {
			scale = 1 // never upscale in "inside" mode
		}
		return scaleTo(src, int(float64(sw)*scale+0.5), int(float64(sh)*scale+0.5))
	}
}

// targetBox resolves the preset's box, filling a missing dimension from aspect.
func targetBox(sw, sh int, p Preset) (int, int) {
	tw, th := p.Width, p.Height
	if tw <= 0 && th <= 0 {
		return sw, sh
	}
	if tw <= 0 {
		tw = int(float64(th) * float64(sw) / float64(sh))
	}
	if th <= 0 {
		th = int(float64(tw) * float64(sh) / float64(sw))
	}
	return tw, th
}

func scaleTo(src image.Image, w, h int) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(out, out.Bounds(), src, src.Bounds(), draw.Over, nil)
	return out
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
func minf(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
