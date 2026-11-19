package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Card is a raised surface with a hairline border. st overrides the
// defaults; the children stack vertically.
func Card(st giode.Styles, children ...giode.Element) giode.Element {
	base := giode.Styles{
		FlexDirection: giode.FlexDirectionColumn,
		Background:    theme.Surface,
		BorderColor:   theme.Faint(),
		BorderWidth:   1,
		BorderRadius:  16,
		Padding:       giode.UniformInset(20),
		Gap:           14,
	}
	return giode.Box(base.With(st), children...)
}
