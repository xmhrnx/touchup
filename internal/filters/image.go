// Package filters cleans up and adjusts pictures and video frames, and
// upscales them without AI, in plain Go: no FFmpeg filters, no GPU and
// no libraries. Clean-up (deinterlacing, deblocking, denoising,
// debanding) runs on the source before any upscaling, where it's cheap
// and keeps an AI model from sharpening the faults; the look (sharpness,
// color, saturation, film grain) is applied to the result.
package filters

import (
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"
)

// Image is a picture as linear rows of RGB floats in [0, 1], in the
// picture's own (sRGB) encoding, with its alpha kept aside.
type Image struct {
	W, H  int
	Pix   []float32 // R, G, B for each pixel, row by row
	Alpha []uint8   // one per pixel, or nil when opaque
}

// NewImage returns a black w×h picture.
func NewImage(w, h int) *Image {
	return &Image{W: w, H: h, Pix: make([]float32, w*h*3)}
}

// FromImage converts any image.
func FromImage(src image.Image) *Image {
	b := src.Bounds()
	// Opaque RGBA, as PNG video frames decode to, is NRGBA already.
	if r, ok := src.(*image.RGBA); ok && r.Rect.Min == (image.Point{}) && r.Opaque() {
		return FromNRGBA(&image.NRGBA{Pix: r.Pix, Stride: r.Stride, Rect: r.Rect})
	}
	n, ok := src.(*image.NRGBA)
	if !ok || n.Rect.Min != (image.Point{}) {
		n = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		draw.Draw(n, n.Bounds(), src, b.Min, draw.Src)
	}
	return FromNRGBA(n)
}

// FromNRGBA converts an NRGBA picture.
func FromNRGBA(n *image.NRGBA) *Image {
	w, h := n.Rect.Dx(), n.Rect.Dy()
	m := NewImage(w, h)
	opaque := true
	rows(h, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			src := n.Pix[y*n.Stride:]
			dst := m.Pix[y*w*3:]
			for x := 0; x < w; x++ {
				dst[x*3] = lut8[src[x*4]]
				dst[x*3+1] = lut8[src[x*4+1]]
				dst[x*3+2] = lut8[src[x*4+2]]
			}
		}
	})
	for i := 3; i < len(n.Pix) && opaque; i += 4 {
		opaque = n.Pix[i] == 255
	}
	if !opaque {
		m.Alpha = make([]uint8, w*h)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				m.Alpha[y*w+x] = n.Pix[y*n.Stride+x*4+3]
			}
		}
	}
	return m
}

// FromRGB24 converts packed 8-bit RGB, as FFmpeg's rgb24 frames are.
func FromRGB24(data []byte, w, h int) *Image {
	m := NewImage(w, h)
	rows(h, func(y0, y1 int) {
		for i := y0 * w * 3; i < y1*w*3; i++ {
			m.Pix[i] = lut8[data[i]]
		}
	})
	return m
}

var lut8 = func() (t [256]float32) {
	for i := range t {
		t[i] = float32(i) / 255
	}
	return
}()

// NRGBA converts back to 8 bits a channel.
func (m *Image) NRGBA() *image.NRGBA {
	n := image.NewNRGBA(image.Rect(0, 0, m.W, m.H))
	rows(m.H, func(y0, y1 int) {
		for y := y0; y < y1; y++ {
			for x := 0; x < m.W; x++ {
				i, o := (y*m.W+x)*3, y*n.Stride+x*4
				n.Pix[o], n.Pix[o+1], n.Pix[o+2] = to8(m.Pix[i]), to8(m.Pix[i+1]), to8(m.Pix[i+2])
				n.Pix[o+3] = 255
				if m.Alpha != nil {
					n.Pix[o+3] = m.Alpha[y*m.W+x]
				}
			}
		}
	})
	return n
}

// RGB24 converts to packed 8-bit RGB, appending to buf.
func (m *Image) RGB24(buf []byte) []byte {
	n := m.W * m.H * 3
	if cap(buf) < n {
		buf = make([]byte, n)
	}
	buf = buf[:n]
	rows(m.H, func(y0, y1 int) {
		for i := y0 * m.W * 3; i < y1*m.W*3; i++ {
			buf[i] = to8(m.Pix[i])
		}
	})
	return buf
}

func to8(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// Clone returns a copy.
func (m *Image) Clone() *Image {
	c := &Image{W: m.W, H: m.H, Pix: append([]float32(nil), m.Pix...)}
	if m.Alpha != nil {
		c.Alpha = append([]uint8(nil), m.Alpha...)
	}
	return c
}

// at returns channel c of the pixel at x, y, with coordinates outside
// the picture clamped to its edge.
func (m *Image) at(x, y, c int) float32 {
	x = clampInt(x, 0, m.W-1)
	y = clampInt(y, 0, m.H-1)
	return m.Pix[(y*m.W+x)*3+c]
}

// luma at x, y, clamped to the edge.
func (m *Image) luma(x, y int) float32 {
	x = clampInt(x, 0, m.W-1)
	y = clampInt(y, 0, m.H-1)
	i := (y*m.W + x) * 3
	return luma(m.Pix[i], m.Pix[i+1], m.Pix[i+2])
}

func luma(r, g, b float32) float32 { return 0.2126*r + 0.7152*g + 0.0722*b }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// rows splits h rows among the CPU's cores and runs fn on each share.
func rows(h int, fn func(y0, y1 int)) {
	n := min(runtime.GOMAXPROCS(0), max(1, h/16))
	if n <= 1 {
		fn(0, h)
		return
	}
	var wg sync.WaitGroup
	step := (h + n - 1) / n
	for y := 0; y < h; y += step {
		wg.Add(1)
		go func(y0, y1 int) {
			defer wg.Done()
			fn(y0, y1)
		}(y, min(h, y+step))
	}
	wg.Wait()
}

// noise returns a repeatable pseudo-random number in [0, 1) for a pixel
// of a frame, so a filter gives the same result for the same frame.
func noise(x, y, frame, salt uint32) float32 {
	h := x*0x9E3779B1 ^ y*0x85EBCA77 ^ frame*0xC2B2AE3D ^ salt*0x27D4EB2F
	h ^= h >> 15
	h *= 0x2C1B3C6D
	h ^= h >> 12
	h *= 0x297A2D39
	h ^= h >> 15
	return float32(h>>8) / (1 << 24)
}

// gauss turns two uniform numbers into a normally distributed one.
func gauss(u1, u2 float32) float32 {
	u1 = max(u1, 1e-7)
	return float32(math.Sqrt(-2*math.Log(float64(u1))) * math.Cos(2*math.Pi*float64(u2)))
}
