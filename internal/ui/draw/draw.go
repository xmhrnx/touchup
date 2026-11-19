// Package draw has the low-level painting helpers TouchUp's custom
// widgets share: rounded rectangles, images fitted into a rectangle and
// colors resolved from theme tokens.
package draw

import (
	"image"
	"image/color"

	"gioui.org/f32"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/xmhrnx/giode/styles/properties"
)

// C resolves a color string, theme tokens included.
func C(c string) color.NRGBA { return properties.ResolveColor(c) }

// WithAlpha returns c with its alpha scaled by a.
func WithAlpha(c color.NRGBA, a float32) color.NRGBA {
	c.A = uint8(float32(c.A) * max(0, min(1, a)))
	return c
}

// Lerp blends from a to b by t.
func Lerp(a, b color.NRGBA, t float32) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float32(x) + (float32(y)-float32(x))*t) }
	return color.NRGBA{l(a.R, b.R), l(a.G, b.G), l(a.B, b.B), l(a.A, b.A)}
}

// RRect fills r with rounded corners of radius px.
func RRect(ops *op.Ops, r image.Rectangle, radius int, c color.NRGBA) {
	paint.FillShape(ops, c, clip.UniformRRect(r, radius).Op(ops))
}

// Rect fills r.
func Rect(ops *op.Ops, r image.Rectangle, c color.NRGBA) {
	paint.FillShape(ops, c, clip.Rect(r).Op())
}

// Circle fills a circle of radius px around center.
func Circle(ops *op.Ops, center image.Point, radius int, c color.NRGBA) {
	r := image.Rect(center.X-radius, center.Y-radius, center.X+radius, center.Y+radius)
	paint.FillShape(ops, c, clip.Ellipse(r).Op(ops))
}

// Image paints img scaled by scale with its top left corner at at,
// clipped to the current clip.
func Image(ops *op.Ops, img image.Image, at f32.Point, scale float32, filter paint.ImageFilter) {
	if img == nil {
		return
	}
	io := properties.ImageOp(img)
	io.Filter = filter
	b := img.Bounds()
	tr := op.Affine(f32.Affine2D{}.
		Offset(f32.Pt(-float32(b.Min.X), -float32(b.Min.Y))).
		Scale(f32.Point{}, f32.Pt(scale, scale)).
		Offset(at)).Push(ops)
	io.Add(ops)
	paint.PaintOp{}.Add(ops)
	tr.Pop()
}

// Cover paints img scaled to cover r, centered and clipped to r.
func Cover(ops *op.Ops, img image.Image, r image.Rectangle) {
	if img == nil || r.Empty() {
		return
	}
	b := img.Bounds()
	scale := max(float32(r.Dx())/float32(b.Dx()), float32(r.Dy())/float32(b.Dy()))
	w, h := float32(b.Dx())*scale, float32(b.Dy())*scale
	at := f32.Pt(float32(r.Min.X)+(float32(r.Dx())-w)/2, float32(r.Min.Y)+(float32(r.Dy())-h)/2)
	cl := clip.Rect(r).Push(ops)
	Image(ops, img, at, scale, paint.FilterLinear)
	cl.Pop()
}
