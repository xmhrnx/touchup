package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Tag is a small tinted label in color.
func Tag(text string, color giode.Color) giode.Element {
	return giode.Box(giode.Styles{
		Background: theme.Tint(color, 0.16), BorderRadius: 6, Padding: giode.SymmetricInset(7, 3),
	}, giode.Text(text, giode.Styles{FontFamily: theme.Sans, FontSize: 11, FontWeight: giode.FontWeightSemiBold, Color: color}))
}
