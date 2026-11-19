package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// MetaChip shows one fact about a file: an icon and a short value.
func MetaChip(icon, text string) giode.Element {
	return giode.Row(giode.Styles{
		Align: giode.AlignCenter, Gap: 6, Wrap: giode.WrapNoWrap,
		Background: theme.Raised(), BorderRadius: 8, Padding: giode.SymmetricInset(9, 5),
	},
		theme.Icon(icon, 13, theme.Muted),
		giode.Text(text, giode.Styles{FontFamily: theme.Sans, FontSize: 12, FontWeight: giode.FontWeightMedium, Color: theme.Text}),
	)
}
