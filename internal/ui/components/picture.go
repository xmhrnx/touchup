package components

import (
	"image"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/draw"
)

// Picture shows an image fitted and centered in the space it's given,
// on a dark stage.
func Picture(img image.Image) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		if img == nil || size.X == 0 || size.Y == 0 {
			return layout.Dimensions{Size: size}
		}
		b := img.Bounds()
		scale := min(float32(size.X)/float32(b.Dx()), float32(size.Y)/float32(b.Dy()))
		w, h := float32(b.Dx())*scale, float32(b.Dy())*scale
		at := f32.Pt((float32(size.X)-w)/2, (float32(size.Y)-h)/2)
		cl := clip.Rect{Max: size}.Push(gtx.Ops)
		draw.Image(gtx.Ops, img, at, scale, paint.FilterLinear)
		cl.Pop()
		return layout.Dimensions{Size: size}
	})
}
