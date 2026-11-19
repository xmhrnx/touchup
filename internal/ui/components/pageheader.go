package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// PageHeader is a page's title and subtitle, with actions on the right.
func PageHeader(title, subtitle string, actions ...giode.Element) giode.Element {
	text := giode.Stack(giode.Styles{Gap: 4, FlexGrow: 1},
		giode.Text(title, theme.Display()),
		giode.Text(subtitle, theme.Secondary()),
	)
	return giode.Row(giode.Styles{Width: -1, Align: giode.AlignCenter, Gap: 10, Wrap: giode.WrapNoWrap},
		append([]giode.Element{text}, actions...)...)
}
