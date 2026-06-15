package menubar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// Status colors for the menu-bar dot (chosen to read at a glance in the bar).
var (
	colorWaiting = color.RGBA{0xEF, 0x44, 0x44, 0xFF} // red — needs you
	colorWorking = color.RGBA{0x06, 0xB6, 0xD4, 0xFF} // cyan — busy
	colorIdle    = color.RGBA{0x22, 0xC5, 0x5E, 0xFF} // green — done, your move
	colorNone    = color.RGBA{0x8E, 0x8E, 0x93, 0xFF} // gray — nothing running
)

// IconColor picks the dot color for the most-urgent state present.
func IconColor(agents []Agent) color.RGBA {
	w, wk := counts(agents)
	switch {
	case w > 0:
		return colorWaiting
	case wk > 0:
		return colorWorking
	case len(agents) > 0:
		return colorIdle
	default:
		return colorNone
	}
}

// IconFor renders the status dot as PNG bytes for systray.SetIcon (non-template,
// so the color shows). A filled, anti-aliased circle on a transparent canvas.
func IconFor(agents []Agent) []byte {
	return dotPNG(IconColor(agents))
}

func dotPNG(c color.RGBA) []byte {
	const size = 22
	const r = 8.0 // dot radius, leaving menu-bar padding
	center := (size - 1) / 2.0
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Distance from the dot edge, used for a 1px anti-aliased rim.
			dx, dy := float64(x)-center, float64(y)-center
			d := r - sqrt(dx*dx+dy*dy)
			switch {
			case d >= 1:
				img.Set(x, y, c)
			case d > 0:
				a := uint8(float64(c.A) * d)
				img.Set(x, y, color.RGBA{c.R, c.G, c.B, a})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// sqrt is a tiny dependency-free square root (Newton's method) — avoids pulling
// math just for the icon rasterizer.
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	z := x
	for i := 0; i < 20; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}
