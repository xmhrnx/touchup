package components

import (
	"image"
	"math"
	"time"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/draw"
	"github.com/mehran9675/touchup/internal/ui/theme"
)

// ProgressBar is a thin bar filled with the brand gradient. A negative
// fraction animates a sweeping segment for work of unknown length.
func ProgressBar(frac float64, height int) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		h := gtx.Dp(unit.Dp(height))
		size := image.Pt(gtx.Constraints.Max.X, h)
		r := image.Rectangle{Max: size}
		draw.RRect(gtx.Ops, r, h/2, draw.C(string(theme.Sunken())))
		var fill image.Rectangle
		if frac < 0 {
			// A segment a third of the width sweeping across.
			t := float64(gtx.Now.UnixMilli()%1400) / 1400
			t = 0.5 - 0.5*math.Cos(t*math.Pi)
			w := size.X / 3
			x := int(t*float64(size.X+w)) - w
			fill = image.Rect(max(0, x), 0, min(size.X, x+w), h)
			gtx.Execute(op.InvalidateCmd{At: gtx.Now.Add(16 * time.Millisecond)})
		} else {
			fill = image.Rect(0, 0, int(float64(size.X)*min(1, frac)), h)
		}
		if fill.Dx() > 0 {
			cl := clip.UniformRRect(fill, h/2).Push(gtx.Ops)
			paint.LinearGradientOp{
				Stop1: f32.Pt(0, 0), Color1: draw.C(theme.Violet),
				Stop2: f32.Pt(float32(max(size.X, 1)), 0), Color2: draw.C(theme.Pink),
			}.Add(gtx.Ops)
			paint.PaintOp{}.Add(gtx.Ops)
			cl.Pop()
		}
		return layout.Dimensions{Size: size}
	})
}
