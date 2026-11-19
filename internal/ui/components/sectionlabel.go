package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// SectionLabel heads a group of controls, with optional elements on the
// right.
func SectionLabel(text string, trailing ...giode.Element) giode.Element {
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Wrap: giode.WrapNoWrap},
		append([]giode.Element{
			giode.Box(giode.Styles{FlexGrow: 1}, giode.Text(text, theme.Overline())),
		}, trailing...)...)
}
