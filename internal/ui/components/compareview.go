package components

import (
	"fmt"
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/mehran9675/giode"
	"github.com/mehran9675/giode/components/iconbutton"

	"github.com/mehran9675/touchup/internal/tr"
	"github.com/mehran9675/touchup/internal/ui/draw"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// CompareView shows two versions of a picture split by a draggable
// divider: the original on the left, the upscaled one on the right. The
// wheel zooms around the pointer and dragging pans; the original is
// stretched to the upscaled size, so both halves line up pixel for pixel.
type CompareView struct {
	before, after           image.Image
	beforeLabel, afterLabel string

	split float32 // divider position, 0..1 of the width
	zoom  float32 // 1 fits the picture in the view
	pan   f32.Point
	drag  int // 0 none, 1 divider, 2 pan
	last  f32.Point
	size  image.Point

	zoomIn, zoomOut, fit *iconbutton.Button
}

const (
	dragNone = iota
	dragSplit
	dragPan
	maxZoom = 16
)

// NewCompareView creates an empty compare view.
func NewCompareView() *CompareView {
	c := &CompareView{split: 0.5, zoom: 1}
	c.zoomIn = theme.IconButton("zoom-in", tr.T("Zoom in")).OnClick(func() { c.zoomBy(1.5, c.center()) })
	c.zoomOut = theme.IconButton("zoom-out", tr.T("Zoom out")).OnClick(func() { c.zoomBy(1/1.5, c.center()) })
	c.fit = theme.IconButton("maximize", tr.T("Fit")).OnClick(func() { c.zoom, c.pan = 1, f32.Point{} })
	return c
}

// Set shows a new pair of pictures. The zoom resets when the pictures
// change size.
func (c *CompareView) Set(before, after image.Image, beforeLabel, afterLabel string) {
	if after != nil && (c.after == nil || after.Bounds().Size() != c.after.Bounds().Size()) {
		c.zoom, c.pan = 1, f32.Point{}
	}
	c.before, c.after = before, after
	c.beforeLabel, c.afterLabel = beforeLabel, afterLabel
}

// Layout draws the view filling the space it's given.
func (c *CompareView) Layout(gtx layout.Context) layout.Dimensions {
	pill := func(text string, style giode.Styles) giode.Element {
		return giode.Box(giode.Styles{
			Background: "#0B0B10CC", BorderRadius: 999, Padding: giode.SymmetricInset(10, 5),
		}, giode.Text(text, style))
	}
	labelStyle := giode.Styles{FontFamily: theme.Sans, FontSize: 11.5, FontWeight: giode.FontWeightSemiBold, Color: "#F4F4F7"}
	zoomText := fmt.Sprintf("%d%%", int(c.zoom*100+0.5))
	return giode.Box(giode.Styles{Width: -1, Height: -1, BorderRadius: 14, Overflow: giode.OverflowHidden, Background: "#050507"},
		giode.Raw(c.canvas),
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Top: 14, Left: 14},
			pill(c.beforeLabel, labelStyle)),
		giode.Box(giode.Styles{Position: giode.PositionAbsolute, Top: 14, Right: 14},
			giode.Box(giode.Styles{BackgroundGradient: theme.BrandGradient(), BorderRadius: 999, Padding: giode.SymmetricInset(10, 5)},
				giode.Text(c.afterLabel, labelStyle))),
		giode.Box(giode.Styles{
			Position: giode.PositionAbsolute, Bottom: 14, Right: 14,
			Background: "#0B0B10CC", BorderRadius: 10, Padding: giode.UniformInset(3),
		},
			giode.Row(giode.Styles{Align: giode.AlignCenter, Gap: 2},
				c.zoomOut,
				giode.Box(giode.Styles{MinWidth: 44}, giode.Text(zoomText, giode.Styles{
					FontFamily: theme.Sans, FontSize: 12, Color: "#D4D4DC", TextAlign: giode.TextAlignCenter,
				})),
				c.zoomIn,
				c.fit,
			)),
	).Layout(gtx)
}

func (c *CompareView) canvas(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Max
	c.size = size
	c.events(gtx)

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, c)
	if c.drag == dragPan {
		pointer.CursorGrabbing.Add(gtx.Ops)
	} else if c.zoom > 1 {
		pointer.CursorGrab.Add(gtx.Ops)
	}
	if c.after == nil {
		return layout.Dimensions{Size: size}
	}

	tl, scale := c.placement()
	ab := c.after.Bounds()
	splitX := int(float32(size.X) * c.split)
	filter := paint.FilterLinear
	if scale >= 3 {
		filter = paint.FilterNearest // show real pixels when zoomed far in
	}

	// Original, stretched to the upscaled size, left of the divider.
	if c.before != nil {
		bb := c.before.Bounds()
		bscale := scale * float32(ab.Dx()) / float32(bb.Dx())
		cl := clip.Rect{Max: image.Pt(splitX, size.Y)}.Push(gtx.Ops)
		draw.Image(gtx.Ops, c.before, tl, bscale, filter)
		cl.Pop()
	}
	// Upscaled, right of the divider.
	cl := clip.Rect{Min: image.Pt(splitX, 0), Max: size}.Push(gtx.Ops)
	draw.Image(gtx.Ops, c.after, tl, scale, filter)
	cl.Pop()

	// Divider and handle.
	white := draw.C("#FFFFFF")
	lineW := max(2, gtx.Dp(2))
	draw.Rect(gtx.Ops, image.Rect(splitX-lineW/2, 0, splitX+lineW-lineW/2, size.Y), draw.WithAlpha(white, 0.9))
	center := image.Pt(splitX, size.Y/2)
	r := gtx.Dp(18)
	draw.Circle(gtx.Ops, center.Add(image.Pt(0, gtx.Dp(2))), r+gtx.Dp(2), draw.C("#00000055"))
	draw.Circle(gtx.Ops, center, r, white)
	c.arrows(gtx, center)
	return layout.Dimensions{Size: size}
}

// arrows draws the ◀ ▶ glyph on the divider handle.
func (c *CompareView) arrows(gtx layout.Context, at image.Point) {
	s := float32(gtx.Dp(5))
	gap := float32(gtx.Dp(3))
	p := f32.Pt(float32(at.X), float32(at.Y))
	var path clip.Path
	path.Begin(gtx.Ops)
	path.MoveTo(p.Add(f32.Pt(-gap, -s)))
	path.LineTo(p.Add(f32.Pt(-gap-s, 0)))
	path.LineTo(p.Add(f32.Pt(-gap, s)))
	path.Close()
	path.MoveTo(p.Add(f32.Pt(gap, -s)))
	path.LineTo(p.Add(f32.Pt(gap+s, 0)))
	path.LineTo(p.Add(f32.Pt(gap, s)))
	path.Close()
	paint.FillShape(gtx.Ops, draw.C(theme.Violet), clip.Outline{Path: path.End()}.Op())
}

// placement returns the top left corner and scale of the upscaled
// picture.
func (c *CompareView) placement() (f32.Point, float32) {
	if c.after == nil || c.size.X == 0 || c.size.Y == 0 {
		return f32.Point{}, 1
	}
	b := c.after.Bounds()
	fit := min(float32(c.size.X)/float32(b.Dx()), float32(c.size.Y)/float32(b.Dy()))
	scale := fit * c.zoom
	w, h := float32(b.Dx())*scale, float32(b.Dy())*scale
	c.clampPan(w, h)
	tl := f32.Pt((float32(c.size.X)-w)/2+c.pan.X, (float32(c.size.Y)-h)/2+c.pan.Y)
	return tl, scale
}

// clampPan keeps the picture from being dragged out of view.
func (c *CompareView) clampPan(w, h float32) {
	mx := max(0, (w-float32(c.size.X))/2)
	my := max(0, (h-float32(c.size.Y))/2)
	c.pan.X = max(-mx, min(mx, c.pan.X))
	c.pan.Y = max(-my, min(my, c.pan.Y))
}

func (c *CompareView) center() f32.Point {
	return f32.Pt(float32(c.size.X)/2, float32(c.size.Y)/2)
}

// zoomBy zooms by factor keeping the picture point under at in place.
func (c *CompareView) zoomBy(factor float32, at f32.Point) {
	if c.after == nil {
		return
	}
	tl, scale := c.placement()
	next := max(1, min(maxZoom, c.zoom*factor))
	if next == c.zoom {
		return
	}
	u := at.Sub(tl).Mul(1 / scale) // picture coordinates under the pointer
	c.zoom = next
	_, nscale := c.placement()
	b := c.after.Bounds()
	w, h := float32(b.Dx())*nscale, float32(b.Dy())*nscale
	ntl := at.Sub(u.Mul(nscale))
	c.pan = f32.Pt(ntl.X-(float32(c.size.X)-w)/2, ntl.Y-(float32(c.size.Y)-h)/2)
}

func (c *CompareView) events(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  c,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Scroll,
			ScrollY: pointer.ScrollRange{Min: -math.MaxInt32, Max: math.MaxInt32},
		})
		if !ok {
			return
		}
		e, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch e.Kind {
		case pointer.Press:
			splitX := float32(c.size.X) * c.split
			c.last = e.Position
			switch {
			case abs32(e.Position.X-splitX) < float32(gtx.Dp(22)) || c.zoom <= 1:
				c.drag = dragSplit
				c.setSplit(e.Position.X)
			default:
				c.drag = dragPan
			}
		case pointer.Drag:
			switch c.drag {
			case dragSplit:
				c.setSplit(e.Position.X)
			case dragPan:
				c.pan = c.pan.Add(e.Position.Sub(c.last))
				c.last = e.Position
			}
		case pointer.Release, pointer.Cancel:
			c.drag = dragNone
		case pointer.Scroll:
			factor := float32(math.Pow(1.0015, float64(-e.Scroll.Y)))
			c.zoomBy(factor, e.Position)
		}
	}
}

func (c *CompareView) setSplit(x float32) {
	if c.size.X > 0 {
		c.split = max(0, min(1, x/float32(c.size.X)))
	}
}
