package components

import (
	"image"
	"math"

	"gioui.org/layout"
	"gioui.org/unit"

	"github.com/xmhrnx/giode"
	"github.com/xmhrnx/giode/elements"
	"github.com/xmhrnx/giode/styles/properties"
)

// Grid lays cells out in as many equal columns as fit at least minWidth
// dp each, with every cell in a row as tall as the tallest. cell builds
// the i-th of n cells at a width in dp, and at a height in dp, or 0 when
// the grid is measuring its natural height (a cell must not stretch
// then).
func Grid(n, minWidth, gap int, cell func(i, width, height int) giode.Element) giode.Element {
	return giode.Raw(func(gtx layout.Context) layout.Dimensions {
		avail := int(gtx.Metric.PxToDp(gtx.Constraints.Max.X))
		cols := max(1, (avail+gap)/(minWidth+gap))
		width := (avail - gap*(cols-1)) / cols

		measure := gtx
		measure.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(unit.Dp(width)), 1<<24)}
		var rows []giode.Element
		for start := 0; start < n; start += cols {
			end := min(n, start+cols)
			tallest := 0
			for i := start; i < end; i++ {
				tallest = max(tallest, elements.Measure(measure, cell(i, width, 0)).Size.Y)
			}
			height := int(math.Ceil(float64(gtx.Metric.PxToDp(tallest))))
			cells := make([]giode.Element, 0, end-start)
			for i := start; i < end; i++ {
				cells = append(cells, cell(i, width, height))
			}
			rows = append(rows, giode.Row(giode.Styles{Gap: properties.Gap(gap), Wrap: giode.WrapNoWrap}, cells...))
		}
		return giode.Stack(giode.Styles{Width: -1, Gap: properties.Gap(gap)}, rows...).Layout(gtx)
	})
}
