package components

import (
	"github.com/xmhrnx/giode"

	"github.com/mehran9675/touchup/internal/ui/theme"
)

// Pill is one choice among several, filled when selected.
func Pill(scope string, key any, label string, selected bool, onClick func()) giode.Element {
	return Clickable(scope, key, label, onClick, func(s giode.PressState) giode.Element {
		bg, border, color := theme.Raised(), giode.Color(theme.Border), giode.Color(theme.Text)
		switch {
		case selected:
			bg, border = theme.Tint(theme.Primary, 0.2), theme.Primary
		case s.Hovered:
			bg = theme.Hover()
		}
		return giode.Box(giode.Styles{
			Background: bg, BorderColor: border, BorderWidth: 1, BorderRadius: 9,
			Padding: giode.SymmetricInset(13, 7),
		}, giode.Text(label, giode.Styles{FontFamily: theme.Sans, FontSize: 12.5, FontWeight: giode.FontWeightSemiBold, Color: color}))
	})
}
