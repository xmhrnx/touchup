package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// StatusBadge is a dot and a word: "Installed", "Running".
func StatusBadge(text string, color giode.Color) giode.Element {
	return giode.Row(giode.Styles{
		Align: giode.AlignCenter, Gap: 6, Wrap: giode.WrapNoWrap,
		Background: theme.Tint(color, 0.14), BorderRadius: 999, Padding: giode.SymmetricInset(9, 4),
	},
		giode.Box(giode.Styles{Width: 6, Height: 6, BorderRadius: 3, Background: color}),
		giode.Text(text, giode.Styles{FontFamily: theme.Sans, FontSize: 11.5, FontWeight: giode.FontWeightSemiBold, Color: color}),
	)
}
